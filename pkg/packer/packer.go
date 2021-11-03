// Package packer implements functions to build and manage images with Packer
package packer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
)

const envHCloudToken = "HCLOUD_TOKEN"

// Packer stores information about packer binary and current packer builds.
type Packer struct {
	log              logr.Logger
	packerConfigPath string

	packerPath string

	buildsLock sync.Mutex
	builds     map[string]*build
}

type build struct {
	*exec.Cmd
	terminated bool
	result     error
	stdout     bytes.Buffer
	stderr     bytes.Buffer
}

func (b *build) Start() error {
	b.Cmd.Stdout = &b.stdout
	b.Cmd.Stderr = &b.stderr
	err := b.Cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		b.result = b.Cmd.Wait()
		b.terminated = true
	}()
	return err
}

// New creates new packer struct.
func New(log logr.Logger) *Packer {
	return &Packer{
		log:    log,
		builds: make(map[string]*build),
	}
}

// Initialize downloads the given tar.gz file and checks packer binary and config.
func (m *Packer) Initialize(machine *infrav1.HCloudMachine) error {
	// if not set packer won't be started
	if machine.Spec.Image.URL == nil {
		return nil
	}

	// check if validate remote URL is specified
	if strings.HasPrefix(*machine.Spec.Image.URL, "http://") || strings.HasPrefix(*machine.Spec.Image.URL, "https://") {
		return fmt.Errorf("no validate remote URL specified: %s", *machine.Spec.Image.URL)
	}

	splitStrings := strings.Split(*machine.Spec.Image.URL, "/")
	folderName := strings.TrimSuffix(splitStrings[len(splitStrings)-1], ".tar.gz")
	_, err := os.Stat(fmt.Sprintf("/tmp/%s", folderName))

	if os.IsNotExist(err) {
		r, err := DownloadFile("my-image.tar.gz", *machine.Spec.Image.URL)
		if err != nil {
			return fmt.Errorf("error while downloading tar.gz file from %s: %s", *machine.Spec.Image.URL, err)
		}

		err = ExtractTarGz(bytes.NewBuffer(r))
		if err != nil {
			return fmt.Errorf("error while getting and unzipping image from %s: %s", *machine.Spec.Image.URL, err)
		}
		splitStrings := strings.Split(*machine.Spec.Image.URL, "/")
		folderName := strings.TrimSuffix(splitStrings[len(splitStrings)-1], ".tar.gz")
		m.packerConfigPath = fmt.Sprintf("/tmp/%s/image.json", folderName)
	}
	if err := m.initializePacker(); err != nil {
		return err
	}
	if err := m.initializeConfig(); err != nil {
		return err
	}
	return nil
}

func (m *Packer) initializePacker() (err error) {
	m.packerPath, err = exec.LookPath("packer")
	if err != nil {
		return fmt.Errorf("error finding packer: %w", err)
	}
	m.log.V(1).Info("packer found in path", "path", m.packerPath)

	// get version of packer
	version, err := m.packerCmd(context.Background(), "-v").Output()
	if err != nil {
		return fmt.Errorf("error executing packer version: %w", err)
	}
	m.log.V(1).Info("packer version", "version", strings.TrimSpace(string(version)))

	return nil
}

func (m *Packer) initializeConfig() (errr error) {
	cmd := m.packerCmd(context.Background(), "validate", m.packerConfigPath)
	cmd.Env = []string{fmt.Sprintf("%s=xxx", envHCloudToken)}
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error validating packer config '%s': %s %w", m.packerConfigPath, string(output), err)
	}
	m.log.V(1).Info("packer config successfully validated", "output", strings.TrimSpace(string(output)))

	return nil
}

func (m *Packer) packerCmd(ctx context.Context, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, m.packerPath, args...) //nolint:gosec
	c.Env = []string{}
	return c
}

// EnsureImage checks if the API has an image already build and if not, it will
// run packer build to create one.
func (m *Packer) EnsureImage(ctx context.Context, log logr.Logger, hc scope.HCloudClient, imageName string) (*infrav1.HCloudImageID, error) {
	key := fmt.Sprintf("%s%s", infrav1.NameHetznerProviderPrefix, "image-name")

	// check if build is currently running
	m.buildsLock.Lock()
	defer m.buildsLock.Unlock()
	if b, ok := m.builds[imageName]; ok {
		// build still running
		if !b.terminated {
			m.log.V(1).Info("packer image build still running", "imageName", imageName)
			return nil, nil
		}

		// check if build has been finished with error
		if err := b.result; err != nil {
			delete(m.builds, imageName)
			return nil, fmt.Errorf("%v stdout=%s stderr=%s", err, b.stdout.String(), b.stderr.String())
		}

		// remove build as it had been successful
		m.log.Info("packer image successfully built", "imageName", imageName)
		delete(m.builds, imageName)
	}

	// query for an existing image by label
	opts := hcloud.ImageListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: fmt.Sprintf("%s==%s", key, imageName),
		},
	}
	imagesByLabel, err := hc.ListImages(ctx, opts)
	if err != nil {
		return nil, err
	}

	// query for an existing image by name
	opts = hcloud.ImageListOpts{
		Name: imageName,
	}
	imagesByName, err := hc.ListImages(ctx, opts)
	if err != nil {
		return nil, err
	}

	images := append(imagesByLabel, imagesByName...)

	var image *hcloud.Image
	for pos := range images {
		i := images[pos]
		if i.Status != hcloud.ImageStatusAvailable {
			continue
		}
		if image == nil || i.Created.After(image.Created) {
			image = i
		}
	}

	// image found, return the latest image
	if image != nil {
		var id = infrav1.HCloudImageID(image.ID)
		return &id, nil
	}

	// schedule build of hcloud image
	b := &build{Cmd: m.packerCmd(context.Background(), "build", m.packerConfigPath)}
	b.Env = []string{fmt.Sprintf("%s=%s", envHCloudToken, hc.Token())}

	m.log.Info("started building packer image", "imageName", imageName)

	if err := b.Start(); err != nil {
		return nil, err
	}

	m.builds[imageName] = b
	return nil, nil
}

// ExtractTarGz extracts a tar gz file.
func ExtractTarGz(gzipStream io.Reader) error {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return fmt.Errorf("ExtractTarGz: NewReader failed: %s", err)
	}

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("ExtractTarGz: Next() failed: %s", err)
		}
		path := "/tmp/" + header.Name
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path, 0750); err != nil {
				return fmt.Errorf("ExtractTarGz: Mkdir() failed: %s", err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Create() failed: %s", err)
			}
			for {
				_, err := io.CopyN(outFile, tarReader, 1024)
				if err != nil {
					if err == io.EOF {
						break
					}
					return fmt.Errorf("ExtractTarGz: Copy() failed: %s", err)
				}
			}
			if err := outFile.Close(); err != nil {
				return errors.Wrap(err, "failed to close file")
			}

		default:
			return fmt.Errorf(
				"extractTarGz: unknown type: %s in %s",
				string(header.Typeflag),
				header.Name)
		}
	}
	log.Info("Extracted File")
	return nil
}

// DownloadFile downloads a file.
func DownloadFile(filepath string, url string) ([]byte, error) {
	// Get the data
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create file in DownloadFile: %s", err)
	}
	log.Info("Downloaded File")
	return body, err
}
