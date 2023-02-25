package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/tilt-dev/clusterid"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	klog "k8s.io/klog/v2"

	cexec "github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/docker"
)

type Machine interface {
	CPUs(ctx context.Context) (int, error)
	EnsureExists(ctx context.Context) error
	Restart(ctx context.Context, desired, existing *api.Cluster) error
}

type unknownMachine struct {
	product clusterid.Product
}

func (m unknownMachine) EnsureExists(ctx context.Context) error {
	return fmt.Errorf("cluster type %s not configurable", m.product)
}

func (m unknownMachine) CPUs(ctx context.Context) (int, error) {
	return 0, nil
}

func (m unknownMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	return fmt.Errorf("cluster type %s not configurable", desired.Product)
}

type sleeper func(dur time.Duration)

type d4mClient interface {
	writeSettings(ctx context.Context, settings map[string]interface{}) error
	settings(ctx context.Context) (map[string]interface{}, error)
	ResetCluster(tx context.Context) error
	setK8sEnabled(settings map[string]interface{}, desired bool) (bool, error)
	ensureMinCPU(settings map[string]interface{}, desired int) (bool, error)
	Open(ctx context.Context) error
}

type dockerMachine struct {
	iostreams    genericclioptions.IOStreams
	dockerClient dockerClient
	sleep        sleeper
	d4m          d4mClient
	os           string
}

func NewDockerMachine(ctx context.Context, client dockerClient, iostreams genericclioptions.IOStreams) (*dockerMachine, error) {
	d4m, err := NewDockerDesktopClient()
	if err != nil {
		return nil, err
	}

	return &dockerMachine{
		dockerClient: client,
		iostreams:    iostreams,
		sleep:        time.Sleep,
		d4m:          d4m,
		os:           runtime.GOOS,
	}, nil
}

func (m *dockerMachine) CPUs(ctx context.Context) (int, error) {
	info, err := m.dockerClient.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.NCPU, nil
}

func (m *dockerMachine) EnsureExists(ctx context.Context) error {
	_, err := m.dockerClient.ServerVersion(ctx)
	if err == nil {
		return nil
	}

	host := m.dockerClient.DaemonHost()

	// If we are connecting to local desktop, we can try to start it.
	// Otherwise, we just error.
	if !docker.IsLocalDockerDesktop(host, m.os) {
		return fmt.Errorf("Not connected to Docker Engine. Host: %q. Error: %v",
			host, err)
	}

	klog.V(2).Infoln("No Docker Desktop running. Attempting to start Docker.")
	err = m.d4m.Open(ctx)
	if err != nil {
		return err
	}

	dur := 60 * time.Second
	_, _ = fmt.Fprintf(m.iostreams.ErrOut, "Waiting %s for Docker Desktop to boot...\n", duration.ShortHumanDuration(dur))
	err = wait.Poll(time.Second, dur, func() (bool, error) {
		_, err := m.dockerClient.ServerVersion(ctx)
		isSuccess := err == nil
		return isSuccess, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for Docker to start")
	}
	klog.V(2).Infoln("Docker started successfully")
	return nil
}

func (m *dockerMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	canChangeCPUs := false
	isLocalDockerDesktop := false
	if docker.IsLocalDockerDesktop(m.dockerClient.DaemonHost(), m.os) {
		canChangeCPUs = true // DockerForMac and DockerForWindows can change the CPU on the VM
		isLocalDockerDesktop = true
	} else if clusterid.Product(desired.Product) == clusterid.ProductMinikube {
		// Minikube can change the CPU on the VM or on the container itself
		canChangeCPUs = true
	}

	if existing.Status.CPUs < desired.MinCPUs && !canChangeCPUs {
		return fmt.Errorf("Cannot automatically set minimum CPU to %d on this platform", desired.MinCPUs)
	}

	if isLocalDockerDesktop {
		settings, err := m.d4m.settings(ctx)
		if err != nil {
			return err
		}

		k8sChanged := false
		if desired.Product == string(clusterid.ProductDockerDesktop) {
			k8sChanged, err = m.d4m.setK8sEnabled(settings, true)
			if err != nil {
				return err
			}
		}

		cpuChanged, err := m.d4m.ensureMinCPU(settings, desired.MinCPUs)
		if err != nil {
			return err
		}

		if k8sChanged || cpuChanged {
			err := m.d4m.writeSettings(ctx, settings)
			if err != nil {
				return err
			}

			dur := 120 * time.Second
			_, _ = fmt.Fprintf(m.iostreams.ErrOut,
				"Applied new Docker Desktop settings. Waiting %s for Docker Desktop to restart...\n",
				duration.ShortHumanDuration(dur))

			// Sleep for short time to ensure the write takes effect.
			m.sleep(2 * time.Second)

			err = wait.Poll(time.Second, dur, func() (bool, error) {
				_, err := m.dockerClient.ServerVersion(ctx)
				isSuccess := err == nil
				return isSuccess, nil
			})
			if err != nil {
				return errors.Wrap(err, "Docker Desktop restart timeout")
			}
		}
	}

	return nil
}

// Currently, out Minikube admin only supports Minikube on Docker,
// so we delegate to the dockerMachine driver.
type minikubeMachine struct {
	iostreams genericclioptions.IOStreams
	runner    cexec.CmdRunner
	dm        *dockerMachine
	name      string
}

func newMinikubeMachine(iostreams genericclioptions.IOStreams, runner cexec.CmdRunner, name string, dm *dockerMachine) *minikubeMachine {
	return &minikubeMachine{
		iostreams: iostreams,
		runner:    runner,
		name:      name,
		dm:        dm,
	}
}

type minikubeSettings struct {
	CPUs int
}

func (m *minikubeMachine) CPUs(ctx context.Context) (int, error) {
	homedir, err := homedir.Dir()
	if err != nil {
		return 0, err
	}
	configPath := filepath.Join(homedir, ".minikube", "profiles", m.name, "config.json")
	f, err := os.Open(configPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	settings := minikubeSettings{}
	err = decoder.Decode(&settings)
	if err != nil {
		return 0, err
	}
	return settings.CPUs, nil
}

func (m *minikubeMachine) EnsureExists(ctx context.Context) error {
	err := m.dm.EnsureExists(ctx)
	if err != nil {
		return err
	}

	m.startIfStopped(ctx)
	return nil
}

func (m *minikubeMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	return m.dm.Restart(ctx, desired, existing)
}

// Minikube is special because the "machine" can be stopped temporarily.
// Check to see if there's a stopped machine, and start it.
// Never return an error - if we can't proceed, we'll just restart from scratch.
func (m *minikubeMachine) startIfStopped(ctx context.Context) {
	out := bytes.NewBuffer(nil)

	// Ignore errors. `minikube status` returns a non-zero exit code when
	// the container has been stopped.
	_ = m.runner.RunIO(ctx, genericclioptions.IOStreams{Out: out, ErrOut: m.iostreams.ErrOut},
		"minikube", "status", "-p", m.name, "-o", "json")

	status := minikubeStatus{}
	decoder := json.NewDecoder(out)
	err := decoder.Decode(&status)
	if err != nil {
		return
	}

	// Handle 'minikube stop'
	if status.Host == "Stopped" {
		_, _ = fmt.Fprintf(m.iostreams.ErrOut, "Cluster %q exists but is stopped. Starting...\n", m.name)
		_ = m.runner.RunIO(ctx, m.iostreams, "minikube", "start", "-p", m.name)
		return
	}

	// Handle 'minikube pause'
	if status.APIServer == "Stopped" {
		_, _ = fmt.Fprintf(m.iostreams.ErrOut, "Cluster %q exists but is paused. Starting...\n", m.name)
		_ = m.runner.RunIO(ctx, m.iostreams, "minikube", "unpause", "-p", m.name)
		return
	}
}

type minikubeStatus struct {
	Host      string
	APIServer string
}
