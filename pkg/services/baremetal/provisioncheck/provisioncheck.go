/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package provisioncheck runs a two-pass rescue/provision reliability check for one HBMH. The bm
// server gets provisioned with a vanilla Ubuntu twice, so we ensure that the machine works and can
// reliably enter the rescue system. It does not need a Kubernetes cluster. The package was create
// to build a standalone cli.
package provisioncheck

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/syself/hrobot-go/models"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hostpkg "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
)

const (
	// DefaultUbuntu2404ImagePath is the stock Ubuntu 24.04 image path exposed by
	// Hetzner's rescue environment for installimage.
	DefaultUbuntu2404ImagePath = "/root/.oldroot/nfs/images/Ubuntu-2404-noble-amd64-base.tar.gz"

	// DefaultPollInterval is the default polling interval for wait steps.
	DefaultPollInterval = 10 * time.Second
	// DefaultLoadInputTimeout is the default timeout for local input loading.
	DefaultLoadInputTimeout = 30 * time.Second
	// DefaultEnsureSSHKeyTimeout is the default timeout for ensuring the Robot SSH key exists.
	DefaultEnsureSSHKeyTimeout = 60 * time.Second
	// DefaultFetchServerDetailsTimeout is the default timeout for fetching server details from Robot.
	DefaultFetchServerDetailsTimeout = 30 * time.Second
	// DefaultActivateRescueTimeout is the default timeout for enabling rescue boot in Robot.
	DefaultActivateRescueTimeout = 45 * time.Second
	// DefaultRebootToRescueTimeout is the default timeout for requesting the reboot into rescue.
	DefaultRebootToRescueTimeout = 45 * time.Second
	// DefaultWaitForRescueTimeout is the default timeout for waiting until rescue SSH is reachable.
	DefaultWaitForRescueTimeout = 6 * time.Minute
	// DefaultCheckDiskInRescueTimeout is the default timeout for smartctl disk checks in rescue.
	DefaultCheckDiskInRescueTimeout = 1 * time.Minute
	// DefaultInstallUbuntuTimeout is the default timeout for one installimage run.
	DefaultInstallUbuntuTimeout = 9 * time.Minute
	// DefaultRebootToOSTimeout is the default timeout for rebooting into the installed OS.
	DefaultRebootToOSTimeout = 45 * time.Second
	// DefaultWaitForOSTimeout is the default timeout for waiting until the installed OS is reachable.
	DefaultWaitForOSTimeout = 6 * time.Minute

	rescueHostName = "rescue"
	sshPort        = 22
)

// Timeouts contains per-step timeouts for the provision check workflow.
type Timeouts struct {
	LoadInput          time.Duration
	EnsureSSHKey       time.Duration
	FetchServerDetails time.Duration
	ActivateRescue     time.Duration
	RebootToRescue     time.Duration
	WaitForRescue      time.Duration
	CheckDiskInRescue  time.Duration
	InstallUbuntu      time.Duration
	RebootToOS         time.Duration
	WaitForOS          time.Duration
}

// Config configures the provision check run.
type Config struct {
	HbmhYAMLFile string
	Name         string
	ImagePath    string
	Force        bool
	PollInterval time.Duration
	Timeouts     Timeouts
	Input        io.Reader
	Output       io.Writer
}

// DefaultConfig returns default configuration values.
func DefaultConfig() Config {
	return Config{
		ImagePath:    DefaultUbuntu2404ImagePath,
		PollInterval: DefaultPollInterval,
		Timeouts: Timeouts{
			LoadInput:          DefaultLoadInputTimeout,
			EnsureSSHKey:       DefaultEnsureSSHKeyTimeout,
			FetchServerDetails: DefaultFetchServerDetailsTimeout,
			ActivateRescue:     DefaultActivateRescueTimeout,
			RebootToRescue:     DefaultRebootToRescueTimeout,
			WaitForRescue:      DefaultWaitForRescueTimeout,
			CheckDiskInRescue:  DefaultCheckDiskInRescueTimeout,
			InstallUbuntu:      DefaultInstallUbuntuTimeout,
			RebootToOS:         DefaultRebootToOSTimeout,
			WaitForOS:          DefaultWaitForOSTimeout,
		},
		Input:  os.Stdin,
		Output: os.Stdout,
	}
}

func (cfg Config) withDefaults() Config {
	defaults := DefaultConfig()

	if cfg.ImagePath == "" {
		cfg.ImagePath = defaults.ImagePath
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = defaults.PollInterval
	}
	if cfg.Input == nil {
		cfg.Input = defaults.Input
	}
	if cfg.Output == nil {
		cfg.Output = defaults.Output
	}

	if cfg.Timeouts.LoadInput == 0 {
		cfg.Timeouts.LoadInput = defaults.Timeouts.LoadInput
	}
	if cfg.Timeouts.EnsureSSHKey == 0 {
		cfg.Timeouts.EnsureSSHKey = defaults.Timeouts.EnsureSSHKey
	}
	if cfg.Timeouts.FetchServerDetails == 0 {
		cfg.Timeouts.FetchServerDetails = defaults.Timeouts.FetchServerDetails
	}
	if cfg.Timeouts.ActivateRescue == 0 {
		cfg.Timeouts.ActivateRescue = defaults.Timeouts.ActivateRescue
	}
	if cfg.Timeouts.RebootToRescue == 0 {
		cfg.Timeouts.RebootToRescue = defaults.Timeouts.RebootToRescue
	}
	if cfg.Timeouts.WaitForRescue == 0 {
		cfg.Timeouts.WaitForRescue = defaults.Timeouts.WaitForRescue
	}
	if cfg.Timeouts.CheckDiskInRescue == 0 {
		cfg.Timeouts.CheckDiskInRescue = defaults.Timeouts.CheckDiskInRescue
	}
	if cfg.Timeouts.InstallUbuntu == 0 {
		cfg.Timeouts.InstallUbuntu = defaults.Timeouts.InstallUbuntu
	}
	if cfg.Timeouts.RebootToOS == 0 {
		cfg.Timeouts.RebootToOS = defaults.Timeouts.RebootToOS
	}
	if cfg.Timeouts.WaitForOS == 0 {
		cfg.Timeouts.WaitForOS = defaults.Timeouts.WaitForOS
	}

	return cfg
}

// Validate checks required inputs and rejects non-positive polling/timeout values.
func (cfg Config) Validate() error {
	if cfg.HbmhYAMLFile == "" {
		return errors.New("--file is required")
	}
	if cfg.ImagePath == "" {
		return errors.New("--image-path must not be empty")
	}
	if cfg.Input == nil {
		return errors.New("config Input must not be nil")
	}
	if cfg.Output == nil {
		return errors.New("config Output must not be nil")
	}
	if cfg.PollInterval <= 0 {
		return fmt.Errorf("--poll-interval must be > 0, got %s", cfg.PollInterval)
	}
	if err := validateTimeout("--timeout-load-input", cfg.Timeouts.LoadInput); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-ensure-ssh-key", cfg.Timeouts.EnsureSSHKey); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-fetch-server", cfg.Timeouts.FetchServerDetails); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-activate-rescue", cfg.Timeouts.ActivateRescue); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-reboot-rescue", cfg.Timeouts.RebootToRescue); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-wait-rescue", cfg.Timeouts.WaitForRescue); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-check-disk-rescue", cfg.Timeouts.CheckDiskInRescue); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-install", cfg.Timeouts.InstallUbuntu); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-reboot-os", cfg.Timeouts.RebootToOS); err != nil {
		return err
	}
	if err := validateTimeout("--timeout-wait-os", cfg.Timeouts.WaitForOS); err != nil {
		return err
	}
	return nil
}

func validateTimeout(flagName string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("%s must be > 0, got %s", flagName, timeout)
	}
	return nil
}

// Run validates the local inputs, selects exactly one HBMH from the YAML file,
// and then executes two destructive rescue/install/boot cycles against that host.
func Run(ctx context.Context, cfg Config) error {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	r := newRunner(cfg)

	// Load all local inputs first so parse and credential errors fail before any
	// Robot API call or reboot on the target machine.
	creds, err := loadEnvCredentials()
	if err != nil {
		return err
	}
	r.creds = creds

	hosts, err := loadHostsFromHBMHYAMLFile(cfg.HbmhYAMLFile)
	if err != nil {
		return err
	}

	r.hostNames = listHostNames(hosts)
	if len(hosts) > 1 && cfg.Name == "" {
		return fmt.Errorf("multiple HBMH objects in file; --name is required. HBMH names: %s", strings.Join(r.hostNames, ", "))
	}

	host, err := selectHost(hosts, cfg.Name)
	if err != nil {
		return err
	}
	r.host = host

	// Ask for confirmation only after we know the exact host and WWNs that will
	// be wiped by the provisioning loop.
	if err := r.confirmDestructiveAction(); err != nil {
		return err
	}

	return r.run(ctx)
}

type runner struct {
	cfg        Config
	in         io.Reader
	out        io.Writer
	sshFactory sshclient.Factory
	startedAt  time.Time
	lastStep   string

	host        infrav1.HetznerBareMetalHost
	hostNames   []string
	creds       envCredentials
	robotClient robotclient.Client
	serverIP    string
	fingerprint string
}

type envCredentials struct {
	robotUser  string
	robotPass  string
	sshKeyName string
	sshPub     string
	sshPriv    string
}

type storageDetails struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	WWN  string `json:"wwn,omitempty"`
}

type stepProgress func(format string, args ...any)

func newRunner(cfg Config) *runner {
	return &runner{
		cfg:        cfg,
		in:         cfg.Input,
		out:        cfg.Output,
		sshFactory: sshclient.NewFactory(),
	}
}

func (r *runner) run(ctx context.Context) error {
	r.startedAt = time.Now()

	err := r.runStep(ctx, "load-input", r.cfg.Timeouts.LoadInput, func(stepCtx context.Context, progress stepProgress) error {
		progress("selected host %q (serverID=%d)", r.host.Name, r.host.Spec.ServerID)
		progress("loaded Robot + SSH credentials from environment")

		_ = stepCtx
		return nil
	})
	if err != nil {
		return err
	}

	r.robotClient = robotclient.NewFactory().NewClient(robotclient.Credentials{
		Username: r.creds.robotUser,
		Password: r.creds.robotPass,
	})

	err = r.runStep(ctx, "ensure-robot-ssh-key", r.cfg.Timeouts.EnsureSSHKey, func(_ context.Context, progress stepProgress) error {
		fingerprint, err := ensureRobotSSHKey(r.robotClient, r.creds.sshKeyName, r.creds.sshPub)
		if err != nil {
			return err
		}
		r.fingerprint = fingerprint
		progress("using robot key=%q fingerprint=%q", r.creds.sshKeyName, r.fingerprint)
		return nil
	})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, "fetch-server-details", r.cfg.Timeouts.FetchServerDetails, func(stepCtx context.Context, progress stepProgress) error {
		return r.refreshServerIP(stepCtx, progress)
	})
	if err != nil {
		return err
	}

	for pass := 1; pass <= 2; pass++ {
		err = r.cycle(ctx, pass)
		if err != nil {
			return err
		}
	}

	r.logf("all checks passed: machine %q (serverID=%d) completed two rescue+install+boot cycles", r.host.Name, r.host.Spec.ServerID)
	return nil
}

func (r *runner) confirmDestructiveAction() error {
	if r.cfg.Force {
		r.logf("confirmation skipped because --force was provided")
		return nil
	}

	rootWWNs := r.host.Spec.RootDeviceHints.ListOfWWN()
	if len(rootWWNs) == 0 {
		return errors.New("rootDeviceHints are required in the input HBMH")
	}

	_, err := fmt.Fprintf(r.out,
		"WARNING: this will delete all data on disks with WWN(s): %s\nhost %q (serverID=%d) \nType \"yes\" to continue: ",
		strings.Join(rootWWNs, ", "),
		r.host.Name,
		r.host.Spec.ServerID,
	)
	if err != nil {
		return fmt.Errorf("write confirmation prompt: %w", err)
	}

	reader := bufio.NewReader(r.in)
	confirmation, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	confirmation = strings.TrimSpace(confirmation)
	if confirmation != "yes" {
		return fmt.Errorf("confirmation failed: expected %q, got %q", "yes", confirmation)
	}

	r.logf("destructive action confirmed for WWN(s): %s", strings.Join(rootWWNs, ", "))
	return nil
}

func (r *runner) cycle(ctx context.Context, pass int) error {
	err := r.runStep(ctx, fmt.Sprintf("pass-%d-activate-rescue", pass), r.cfg.Timeouts.ActivateRescue,
		func(stepCtx context.Context, progress stepProgress) error {
			_ = stepCtx
			_, deleteBootRescueErr := r.robotClient.DeleteBootRescue(r.host.Spec.ServerID)
			if deleteBootRescueErr != nil && !models.IsError(deleteBootRescueErr, models.ErrorCodeNotFound) {
				return fmt.Errorf("delete boot rescue: %w", deleteBootRescueErr)
			}
			_, setBootRescueErr := r.robotClient.SetBootRescue(r.host.Spec.ServerID, r.fingerprint)
			if setBootRescueErr != nil {
				return fmt.Errorf("set boot rescue: %w", setBootRescueErr)
			}
			progress("rescue mode activated")
			return nil
		})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, fmt.Sprintf("pass-%d-reboot-to-rescue", pass), r.cfg.Timeouts.RebootToRescue,
		func(stepCtx context.Context, progress stepProgress) error {
			_ = stepCtx
			_, rebootErr := r.robotClient.RebootBMServer(r.host.Spec.ServerID, infrav1.RebootTypeHardware)
			if rebootErr != nil {
				return fmt.Errorf("robot reboot hw: %w", rebootErr)
			}
			progress("hardware reboot requested")
			return nil
		})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, fmt.Sprintf("pass-%d-wait-rescue", pass), r.cfg.Timeouts.WaitForRescue,
		func(stepCtx context.Context, progress stepProgress) error {
			ssh := r.newRescueSSHClient()
			return waitUntil(stepCtx, r.cfg.PollInterval, progress, func() (bool, string, error) {
				out := ssh.GetHostName()
				if out.Err == nil {
					hostName := strings.TrimSpace(out.StdOut)
					if hostName == rescueHostName {
						return true, fmt.Sprintf("rescue reachable (hostname=%q)", hostName), nil
					}
					if hostName == "" {
						return false, "connected but empty hostname", nil
					}
					return false, fmt.Sprintf("host reachable but hostname=%q (want=%q)", hostName, rescueHostName), nil
				}
				return false, fmt.Sprintf("waiting for rescue ssh: %v", out.Err), nil
			})
		})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, fmt.Sprintf("pass-%d-check-disk-in-rescue", pass), r.cfg.Timeouts.CheckDiskInRescue,
		func(stepCtx context.Context, progress stepProgress) error {
			ssh := r.newRescueSSHClient()
			return r.checkDiskInRescue(stepCtx, ssh, progress)
		})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, fmt.Sprintf("pass-%d-install-ubuntu-24.04", pass), r.cfg.Timeouts.InstallUbuntu,
		func(stepCtx context.Context, progress stepProgress) error {
			ssh := r.newRescueSSHClient()
			return r.runInstall(stepCtx, ssh, progress)
		})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, fmt.Sprintf("pass-%d-reboot-to-os", pass), r.cfg.Timeouts.RebootToOS,
		func(stepCtx context.Context, progress stepProgress) error {
			_ = stepCtx
			out := r.newRescueSSHClient().Reboot()
			if out.Err != nil {
				return fmt.Errorf("reboot via rescue ssh: %w", out.Err)
			}
			progress("reboot command sent from rescue")
			return nil
		})
	if err != nil {
		return err
	}

	err = r.runStep(ctx, fmt.Sprintf("pass-%d-wait-os", pass), r.cfg.Timeouts.WaitForOS,
		func(stepCtx context.Context, progress stepProgress) error {
			ssh := r.newOSSSHClient()
			return waitUntil(stepCtx, r.cfg.PollInterval, progress, func() (bool, string, error) {
				out := ssh.GetHostName()
				if out.Err == nil {
					hostName := strings.TrimSpace(out.StdOut)
					if hostName == "" {
						return false, "os ssh responded with empty hostname", nil
					}
					if hostName == rescueHostName {
						return false, "still in rescue system", nil
					}
					return true, fmt.Sprintf("os reachable (hostname=%q)", hostName), nil
				}
				return false, fmt.Sprintf("waiting for os ssh: %v", out.Err), nil
			})
		})
	if err != nil {
		return err
	}

	return nil
}

func (r *runner) refreshServerIP(_ context.Context, progress stepProgress) error {
	server, err := r.robotClient.GetBMServer(r.host.Spec.ServerID)
	if err != nil {
		return fmt.Errorf("get robot server %d: %w", r.host.Spec.ServerID, err)
	}
	if server.ServerIP == "" {
		return fmt.Errorf("server %d has empty server_ip in Robot API", r.host.Spec.ServerID)
	}
	r.serverIP = server.ServerIP
	progress("server ip=%s", r.serverIP)
	return nil
}

func (r *runner) newRescueSSHClient() sshclient.Client {
	return r.sshFactory.NewClient(sshclient.Input{
		IP:         r.serverIP,
		Port:       sshPort,
		PrivateKey: r.creds.sshPriv,
	})
}

func (r *runner) newOSSSHClient() sshclient.Client {
	return r.sshFactory.NewClient(sshclient.Input{
		IP:         r.serverIP,
		Port:       sshPort,
		PrivateKey: r.creds.sshPriv,
	})
}

func (r *runner) runInstall(ctx context.Context, ssh sshclient.Client, progress stepProgress) error {
	out := ssh.GetHostName()
	if out.Err != nil {
		return fmt.Errorf("rescue ssh check failed: %w", out.Err)
	}
	got := strings.TrimSpace(out.StdOut)
	if got != rescueHostName {
		return fmt.Errorf("expected rescue hostname %q before install, got %q", rescueHostName, got)
	}

	rootWWNs := r.host.Spec.RootDeviceHints.ListOfWWN()
	if len(rootWWNs) == 0 {
		return errors.New("rootDeviceHints are required in the input HBMH")
	}

	devices, err := findDeviceNamesByWWN(ssh, rootWWNs)
	if err != nil {
		return err
	}
	progress("install target devices: %s", strings.Join(devices, ","))

	installSpec := buildInstallImageSpecFromHBMH(r.host, r.cfg.ImagePath, len(rootWWNs) > 1)
	autosetup := buildAutoSetup(installSpec, r.host.Name, devices)
	autoSetupOut := ssh.CreateAutoSetup(autosetup)
	if autoSetupOut.Err != nil || autoSetupOut.StdErr != "" {
		return fmt.Errorf("create autosetup failed: stdout=%q stderr=%q err=%v", autoSetupOut.StdOut, autoSetupOut.StdErr, autoSetupOut.Err)
	}
	progress("autosetup uploaded")

	postInstall := buildPostInstallScript(strings.TrimSpace(r.creds.sshPub))
	postInstallOut := ssh.CreatePostInstallScript(postInstall)
	if postInstallOut.Err != nil || postInstallOut.StdErr != "" {
		return fmt.Errorf("create post-install script failed: stdout=%q stderr=%q err=%v", postInstallOut.StdOut, postInstallOut.StdErr, postInstallOut.Err)
	}
	progress("post-install script uploaded")

	err = ensureInstallImageBinary(ssh, progress)
	if err != nil {
		return err
	}

	started := false
	return waitUntil(ctx, r.cfg.PollInterval, progress, func() (bool, string, error) {
		state, err := ssh.GetInstallImageState()
		if err != nil {
			return false, "", fmt.Errorf("get installimage state: %w", err)
		}

		switch state {
		case sshclient.InstallImageStateRunning:
			return false, "installimage is still running", nil
		case sshclient.InstallImageStateNotStartedYet:
			if !started {
				out := ssh.ExecuteInstallImage(true)
				if out.Err != nil || out.StdErr != "" {
					return false, "", fmt.Errorf("start installimage failed: stdout=%q stderr=%q err=%v", out.StdOut, out.StdErr, out.Err)
				}
				started = true
				return false, "installimage started", nil
			}
			return false, "installimage not started yet", nil
		case sshclient.InstallImageStateFinished:
			result, err := ssh.GetResultOfInstallImage()
			if err != nil {
				logs, logErr := collectInstallLogs(ctx, ssh)
				if logErr != nil {
					return false, "", fmt.Errorf("read installimage result: %w (failed to collect logs: %v)", err, logErr)
				}
				return false, "", fmt.Errorf("read installimage result: %w\ncollected install logs:\n%s", err, logs)
			}
			if !strings.Contains(result, hostpkg.PostInstallScriptFinished) {
				return false, "", fmt.Errorf("installimage finished without marker %q", hostpkg.PostInstallScriptFinished)
			}
			return true, "installimage finished and marker found", nil
		default:
			return false, "", fmt.Errorf("unknown installimage state %q", state)
		}
	})
}

func (r *runner) checkDiskInRescue(ctx context.Context, ssh sshclient.Client, progress stepProgress) error {
	rootWWNs := r.host.Spec.RootDeviceHints.ListOfWWN()
	if len(rootWWNs) == 0 {
		return errors.New("rootDeviceHints are required in the input HBMH")
	}

	diskInfo, err := ssh.CheckDisk(ctx, rootWWNs)
	if err != nil {
		return fmt.Errorf("check-disk failed: %w", err)
	}

	progress("check-disk ok: %s", strings.TrimSpace(diskInfo))
	return nil
}

func collectInstallLogs(ctx context.Context, ssh sshclient.Client) (string, error) {
	script := `#!/bin/bash
set +e

echo "===== debug.txt ====="
cat /root/debug.txt 2>&1 || true
echo
echo "===== installimage-wrapper.sh.log ====="
cat /root/installimage-wrapper.sh.log 2>&1 || true
echo
echo "===== installimage ps ====="
ps aux | grep installimage | grep -v grep || true
`
	tmpFile, err := os.CreateTemp("", "hbmh-provision-check-install-log-*.sh")
	if err != nil {
		return "", fmt.Errorf("create temp script for log collection: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// #nosec G703 -- tmpPath is generated by os.CreateTemp and stays within the local temp dir.
		_ = os.Remove(tmpPath)
	}()
	_, err = tmpFile.WriteString(script)
	if err != nil {
		return "", fmt.Errorf("write temp script for log collection: %w", err)
	}
	err = tmpFile.Close()
	if err != nil {
		return "", fmt.Errorf("close temp script for log collection: %w", err)
	}

	exitStatus, output, err := ssh.ExecutePreProvisionCommand(ctx, tmpPath)
	if err != nil {
		return "", fmt.Errorf("collect install logs via ssh: %w", err)
	}
	if exitStatus != 0 {
		return "", fmt.Errorf("collect install logs exit=%d output=%q", exitStatus, output)
	}
	return output, nil
}

func ensureInstallImageBinary(ssh sshclient.Client, progress stepProgress) error {
	// The CLI registers an embedded installimage tgz via
	// sshclient.SetInstallImageTGZOverride before calling Run. UntarTGZ uploads
	// and extracts that archive in rescue, so this workflow can execute the same
	// installimage tooling without requiring extra local files beside the HBMH
	// YAML manifest.
	out := ssh.UntarTGZ()
	if out.Err != nil || out.StdErr != "" {
		return fmt.Errorf("untar installimage tgz failed: stdout=%q stderr=%q err=%v", out.StdOut, out.StdErr, out.Err)
	}

	progress("installimage files uploaded")
	return nil
}

func ensureRobotSSHKey(cli robotclient.Client, keyName, publicKey string) (string, error) {
	keys, err := cli.ListSSHKeys()
	if err != nil {
		return "", fmt.Errorf("list ssh keys: %w", err)
	}
	for _, key := range keys {
		if key.Name == keyName {
			return key.Fingerprint, nil
		}
	}

	created, err := cli.SetSSHKey(keyName, publicKey)
	if err != nil {
		return "", fmt.Errorf("create ssh key %q: %w", keyName, err)
	}
	return created.Fingerprint, nil
}

func loadHostsFromHBMHYAMLFile(path string) ([]infrav1.HetznerBareMetalHost, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- file path is intentionally user-provided CLI input.
	if err != nil {
		return nil, fmt.Errorf("read yaml file %q: %w", path, err)
	}

	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	result := make([]infrav1.HetznerBareMetalHost, 0)
	for {
		var host infrav1.HetznerBareMetalHost
		err := dec.Decode(&host)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode yaml file %q: %w", path, err)
		}
		if host.Kind != "HetznerBareMetalHost" {
			continue
		}
		result = append(result, *host.DeepCopy())
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no HetznerBareMetalHost objects found in %q", path)
	}

	return result, nil
}

func selectHost(hosts []infrav1.HetznerBareMetalHost, name string) (infrav1.HetznerBareMetalHost, error) {
	if name != "" {
		for _, host := range hosts {
			if host.Name == name {
				if host.Spec.RootDeviceHints == nil {
					return infrav1.HetznerBareMetalHost{}, fmt.Errorf("host %q has no spec.rootDeviceHints", host.Name)
				}
				return host, nil
			}
		}
		names := make([]string, 0, len(hosts))
		for _, host := range hosts {
			names = append(names, host.Name)
		}
		sort.Strings(names)
		return infrav1.HetznerBareMetalHost{}, fmt.Errorf("host %q not found in file; available: %s", name, strings.Join(names, ", "))
	}

	if len(hosts) != 1 {
		names := make([]string, 0, len(hosts))
		for _, host := range hosts {
			names = append(names, host.Name)
		}
		sort.Strings(names)
		return infrav1.HetznerBareMetalHost{}, fmt.Errorf("file contains %d hosts; provide --name. available: %s", len(hosts), strings.Join(names, ", "))
	}

	host := hosts[0]
	if host.Spec.RootDeviceHints == nil {
		return infrav1.HetznerBareMetalHost{}, fmt.Errorf("host %q has no spec.rootDeviceHints", host.Name)
	}
	return host, nil
}

func listHostNames(hosts []infrav1.HetznerBareMetalHost) []string {
	names := make([]string, 0, len(hosts))
	for _, host := range hosts {
		names = append(names, host.Name)
	}
	sort.Strings(names)
	return names
}

func loadEnvCredentials() (envCredentials, error) {
	user := strings.TrimSpace(os.Getenv("HETZNER_ROBOT_USER"))
	pass := strings.TrimSpace(os.Getenv("HETZNER_ROBOT_PASSWORD"))
	if user == "" || pass == "" {
		return envCredentials{}, errors.New("HETZNER_ROBOT_USER and HETZNER_ROBOT_PASSWORD are required")
	}

	keyName := strings.TrimSpace(os.Getenv("SSH_KEY_NAME"))
	if keyName == "" {
		return envCredentials{}, errors.New("SSH_KEY_NAME is required")
	}

	sshPub, err := loadKeyMaterial("HETZNER_SSH_PUB_PATH", "HETZNER_SSH_PUB")
	if err != nil {
		return envCredentials{}, fmt.Errorf("load public key: %w", err)
	}
	sshPriv, err := loadKeyMaterial("HETZNER_SSH_PRIV_PATH", "HETZNER_SSH_PRIV")
	if err != nil {
		return envCredentials{}, fmt.Errorf("load private key: %w", err)
	}

	return envCredentials{
		robotUser:  user,
		robotPass:  pass,
		sshKeyName: keyName,
		sshPub:     strings.TrimSpace(sshPub),
		sshPriv:    strings.TrimSpace(sshPriv),
	}, nil
}

func loadKeyMaterial(pathVar, base64Var string) (string, error) {
	path := strings.TrimSpace(os.Getenv(pathVar))
	if path != "" {
		data, err := os.ReadFile(path) // #nosec G304,G703 -- file path is intentionally provided via environment variable.
		if err != nil {
			return "", fmt.Errorf("read %s (%s): %w", pathVar, path, err)
		}
		if len(data) == 0 {
			return "", fmt.Errorf("%s points to empty file: %s", pathVar, path)
		}
		return string(data), nil
	}

	raw := strings.TrimSpace(os.Getenv(base64Var))
	if raw == "" {
		return "", fmt.Errorf("set either %s or %s", pathVar, base64Var)
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		if len(decoded) == 0 {
			return "", fmt.Errorf("%s decoded to empty value", base64Var)
		}
		return string(decoded), nil
	}

	// fallback for plain (already decoded) env values.
	return raw, nil
}

func findDeviceNamesByWWN(ssh sshclient.Client, wwns []string) ([]string, error) {
	out := ssh.GetHardwareDetailsStorage()
	if out.Err != nil {
		return nil, fmt.Errorf("get hardware details storage: %w", out.Err)
	}
	if strings.TrimSpace(out.StdOut) == "" {
		return nil, errors.New("storage output is empty")
	}

	lines := strings.Split(strings.TrimSpace(out.StdOut), "\n")
	storages := make([]storageDetails, 0, len(lines))
	for _, line := range lines {
		var s storageDetails
		validJSON := validJSONFromSSHOutput(line)
		err := json.Unmarshal([]byte(validJSON), &s)
		if err != nil {
			return nil, fmt.Errorf("parse lsblk line %q: %w", line, err)
		}
		if s.Type == "disk" {
			storages = append(storages, s)
		}
	}

	byWWN := make(map[string]string, len(storages))
	for _, storage := range storages {
		wwn := normalizeWWN(storage.WWN)
		if wwn == "" {
			continue
		}
		byWWN[wwn] = storage.Name
	}

	deviceNames := make([]string, 0, len(wwns))
	missing := make([]string, 0)
	for _, wantWWN := range wwns {
		name, ok := byWWN[normalizeWWN(wantWWN)]
		if !ok {
			missing = append(missing, wantWWN)
			continue
		}
		deviceNames = append(deviceNames, name)
	}

	if len(missing) > 0 {
		available := make([]string, 0, len(byWWN))
		for wwn, name := range byWWN {
			available = append(available, fmt.Sprintf("%s=%s", wwn, name))
		}
		sort.Strings(available)
		return nil, fmt.Errorf("failed to map rootDeviceHints WWN(s) to disk names: missing=%v available=%v", missing, available)
	}
	return deviceNames, nil
}

func normalizeWWN(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func buildPostInstallScript(publicKey string) string {
	return fmt.Sprintf(`#!/bin/bash
set -Eeuo pipefail

mkdir -p /etc/cloud/cloud.cfg.d && touch /etc/cloud/cloud.cfg.d/99-custom-networking.cfg
echo "network: { config: disabled }" > /etc/cloud/cloud.cfg.d/99-custom-networking.cfg
apt-get update && apt-get install -y cloud-init apparmor apparmor-utils
cloud-init clean --logs

install -m 700 -d /root/.ssh
cat << 'EOF_SSH_KEY' > /root/.ssh/authorized_keys
%s
EOF_SSH_KEY
chmod 600 /root/.ssh/authorized_keys

echo %q
`, publicKey, hostpkg.PostInstallScriptFinished)
}

func buildAutoSetup(spec *infrav1.InstallImage, hostName string, osDevices []string) string {
	drives := make([]string, 0, len(osDevices))
	for idx, dev := range osDevices {
		drives = append(drives, fmt.Sprintf("DRIVE%d /dev/%s", idx+1, dev))
	}

	lines := make([]string, 0, 16)
	lines = append(lines,
		strings.Join(drives, "\n"),
		fmt.Sprintf("HOSTNAME %s", hostName),
		fmt.Sprintf("SWRAID %d", spec.Swraid),
	)
	if spec.Swraid == 1 {
		lines = append(lines, fmt.Sprintf("SWRAIDLEVEL %d", spec.SwraidLevel))
	}

	for _, partition := range spec.Partitions {
		lines = append(lines, fmt.Sprintf("PART %s %s %s", partition.Mount, partition.FileSystem, partition.Size))
	}
	for _, lvm := range spec.LVMDefinitions {
		lines = append(lines, fmt.Sprintf("LV %s %s %s %s %s", lvm.VG, lvm.Name, lvm.Mount, lvm.FileSystem, lvm.Size))
	}
	for _, btrfs := range spec.BTRFSDefinitions {
		lines = append(lines, fmt.Sprintf("SUBVOL %s %s %s", btrfs.Volume, btrfs.SubVolume, btrfs.Mount))
	}

	lines = append(lines, fmt.Sprintf("IMAGE %s", spec.Image.Path))
	return strings.Join(lines, "\n")
}

func buildInstallImageSpecFromHBMH(host infrav1.HetznerBareMetalHost, imagePath string, enableSWRAID bool) *infrav1.InstallImage {
	// Prefer the installimage settings already materialized on the HBMH status,
	// because that mirrors what the controller would use for this server. When
	// status.installImage is still empty, fall back to a minimal
	// layout so the standalone checker can still provision the machine.
	if host.Spec.Status.InstallImage == nil {
		return defaultInstallImageSpec(imagePath, enableSWRAID)
	}

	spec := host.Spec.Status.InstallImage.DeepCopy()
	if spec.Image.Path == "" {
		spec.Image.Path = imagePath
	}
	if len(spec.Partitions) == 0 {
		spec.Partitions = defaultInstallImageSpec(imagePath, enableSWRAID).Partitions
	}
	if spec.Swraid != 0 && spec.Swraid != 1 {
		spec.Swraid = 0
	}
	if spec.SwraidLevel == 0 {
		spec.SwraidLevel = 1
	}
	return spec
}

func defaultInstallImageSpec(imagePath string, enableSWRAID bool) *infrav1.InstallImage {
	swraid := 0
	if enableSWRAID {
		swraid = 1
	}

	return &infrav1.InstallImage{
		Image: infrav1.Image{
			Path: imagePath,
		},
		Swraid:      swraid,
		SwraidLevel: 1,
		Partitions: []infrav1.Partition{
			{Mount: "/boot/efi", FileSystem: "esp", Size: "512M"},
			{Mount: "/boot", FileSystem: "ext4", Size: "1024M"},
			{Mount: "/", FileSystem: "ext4", Size: "all"},
		},
	}
}

func validJSONFromSSHOutput(str string) string {
	tempString1 := strings.ReplaceAll(str, `" `, `","`)
	tempString2 := strings.ReplaceAll(tempString1, `="`, `":"`)
	return fmt.Sprintf(`{"%s}`, strings.TrimSpace(tempString2))
}

func waitUntil(ctx context.Context, pollInterval time.Duration, progress stepProgress, check func() (done bool, message string, err error)) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		done, message, err := check()
		if err != nil {
			return err
		}
		if message != "" {
			progress("%s", message)
		}
		if done {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *runner) runStep(ctx context.Context, name string, timeout time.Duration, fn func(context.Context, stepProgress) error) error {
	if timeout <= 0 {
		return fmt.Errorf("step %q has non-positive timeout %s", name, timeout)
	}

	if r.lastStep != "" && r.lastStep != name {
		_, _ = fmt.Fprintln(r.out)
	}
	r.lastStep = name

	start := time.Now()
	r.logf("step=%s state=start timeout=%s", name, formatMinSec(timeout))

	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	progress := func(format string, args ...any) {
		elapsed := time.Since(start)
		remaining := timeout - elapsed
		if remaining < 0 {
			remaining = 0
		}
		used := float64(elapsed) / float64(timeout) * 100
		if used > 100 {
			used = 100
		}
		msg := fmt.Sprintf(format, args...)
		r.logf("step=%s state=running elapsed=%s used=%.1f%% remaining=%s %s",
			name, formatMinSec(elapsed), used, formatMinSec(remaining), msg)
	}

	err := fn(stepCtx, progress)
	duration := time.Since(start)
	remaining := timeout - duration
	if remaining < 0 {
		remaining = 0
	}
	used := float64(duration) / float64(timeout) * 100
	if used > 100 {
		used = 100
	}

	if err == nil {
		r.logf("step=%s state=success duration=%s used=%.1f%% remaining=%s",
			name, formatMinSec(duration), used, formatMinSec(remaining))
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(stepCtx.Err(), context.DeadlineExceeded) {
		r.logf("step=%s state=timeout duration=%s used=%.1f%% remaining=%s err=%v",
			name, formatMinSec(duration), used, formatMinSec(0), err)
		return fmt.Errorf("step %q timed out after %s: %w", name, timeout, err)
	}

	r.logf("step=%s state=failed duration=%s used=%.1f%% remaining=%s err=%v",
		name, formatMinSec(duration), used, formatMinSec(remaining), err)
	return fmt.Errorf("step %q failed: %w", name, err)
}

func (r *runner) logf(format string, args ...any) {
	overall := formatMinSec(0)
	if !r.startedAt.IsZero() {
		overall = formatMinSec(time.Since(r.startedAt))
	}
	_, _ = fmt.Fprintf(r.out, "overall=%s %s\n", overall, fmt.Sprintf(format, args...))
}

func formatMinSec(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second).Seconds())
	minutes := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%dm%02ds", minutes, secs)
}

// ParseServerIDFromName extracts the numeric server ID from names like bm-e2e-1751550.
func ParseServerIDFromName(name string) (int, error) {
	parts := strings.Split(name, "-")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid name %q", name)
	}
	last := parts[len(parts)-1]
	id, err := strconv.Atoi(last)
	if err != nil {
		return 0, fmt.Errorf("parse server id from %q: %w", name, err)
	}
	return id, nil
}
