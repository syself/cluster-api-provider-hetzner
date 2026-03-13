// Command hbmh-provision-check validates rescue/provision reliability for one HBMH from a local YAML file.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/syself/cluster-api-provider-hetzner/data"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioncheck"
)

func main() {
	ctrlLog.SetLogger(logr.Discard())
	data.RegisterEmbeddedInstallImageTGZ()

	// Recreate default flag set to avoid unrelated global flags from imported packages.
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	pflag.Usage = func() {
		out := pflag.CommandLine.Output()
		_, _ = fmt.Fprintf(out, "Usage: %s [flags]\n\n", os.Args[0])
		_, _ = fmt.Fprintln(out, "Validates rescue/provision reliability for one HetznerBareMetalHost from a local YAML file.")
		_, _ = fmt.Fprintln(out, "The tool does not talk to Kubernetes. It only reads the HBMH manifest from --file.")
		_, _ = fmt.Fprintln(out, "You can use a checked-in manifest or export one first, for example:")
		_, _ = fmt.Fprintln(out, "  kubectl get hetznerbaremetalhost <name> -o yaml > host.yaml")
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Required environment variables:")
		_, _ = fmt.Fprintln(out, "  HETZNER_ROBOT_USER        Hetzner Robot username")
		_, _ = fmt.Fprintln(out, "  HETZNER_ROBOT_PASSWORD    Hetzner Robot password")
		_, _ = fmt.Fprintln(out, "  SSH_KEY_NAME              Robot SSH key name to use for rescue mode")
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "SSH key material: set either the *_PATH or the raw/base64 env var")
		_, _ = fmt.Fprintln(out, "  HETZNER_SSH_PUB_PATH or HETZNER_SSH_PUB")
		_, _ = fmt.Fprintln(out, "  HETZNER_SSH_PRIV_PATH or HETZNER_SSH_PRIV")
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "The tool does not talk to Kubernetes. It only reads one local YAML file containing `HetznerBareMetalHost` objects and then talks to Robot plus the target server directly, so no kubeconfig or running cluster is required while you execute it.")
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Flags:")
		pflag.PrintDefaults()
	}

	cfg := provisioncheck.Config{
		Input:  os.Stdin,
		Output: os.Stdout,
	}

	pflag.StringVar(&cfg.HbmhYAMLFile, "file", "", "Path to a local YAML file containing HetznerBareMetalHost objects (required)")
	pflag.StringVar(&cfg.Name, "name", "", "HetznerBareMetalHost metadata.name. Optional if YAML contains exactly one host")
	pflag.StringVar(&cfg.ImagePath, "image-path", provisioncheck.DefaultUbuntu2404ImagePath, "Installimage IMAGE path for Ubuntu 24.04 inside the Hetzner rescue system")
	pflag.BoolVar(&cfg.Force, "force", false, "Skip the destructive-action confirmation prompt")

	pflag.DurationVar(&cfg.PollInterval, "poll-interval", provisioncheck.DefaultPollInterval, "Polling interval for wait steps")
	pflag.DurationVar(&cfg.Timeouts.LoadInput, "timeout-load-input", provisioncheck.DefaultLoadInputTimeout, "Timeout for input parsing + env loading")
	pflag.DurationVar(&cfg.Timeouts.EnsureSSHKey, "timeout-ensure-ssh-key", provisioncheck.DefaultEnsureSSHKeyTimeout, "Timeout for ensuring SSH key in Robot")
	pflag.DurationVar(&cfg.Timeouts.FetchServerDetails, "timeout-fetch-server", provisioncheck.DefaultFetchServerDetailsTimeout, "Timeout for fetching server details from Robot")
	pflag.DurationVar(&cfg.Timeouts.ActivateRescue, "timeout-activate-rescue", provisioncheck.DefaultActivateRescueTimeout, "Timeout for activating rescue boot")
	pflag.DurationVar(&cfg.Timeouts.RebootToRescue, "timeout-reboot-rescue", provisioncheck.DefaultRebootToRescueTimeout, "Timeout for requesting reboot to rescue")
	pflag.DurationVar(&cfg.Timeouts.WaitForRescue, "timeout-wait-rescue", provisioncheck.DefaultWaitForRescueTimeout, "Timeout for waiting until rescue SSH is reachable")
	pflag.DurationVar(&cfg.Timeouts.CheckDiskInRescue, "timeout-check-disk-rescue", provisioncheck.DefaultCheckDiskInRescueTimeout, "Timeout for checking target disks in rescue")
	pflag.DurationVar(&cfg.Timeouts.InstallUbuntu, "timeout-install", provisioncheck.DefaultInstallUbuntuTimeout, "Timeout for one Ubuntu install step")
	pflag.DurationVar(&cfg.Timeouts.RebootToOS, "timeout-reboot-os", provisioncheck.DefaultRebootToOSTimeout, "Timeout for rebooting into installed OS")
	pflag.DurationVar(&cfg.Timeouts.WaitForOS, "timeout-wait-os", provisioncheck.DefaultWaitForOSTimeout, "Timeout for waiting until installed OS is reachable")

	pflag.Parse()

	if cfg.HbmhYAMLFile == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		os.Exit(2)
	}

	// Resolve the file once up front so later logs and errors always refer to a
	// stable absolute path, even when the caller provided a relative one.
	resolved, err := filepath.Abs(cfg.HbmhYAMLFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve --file: %v\n", err)
		os.Exit(2)
	}
	cfg.HbmhYAMLFile = resolved

	if err := provisioncheck.Run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "hbmh-provision-check failed. %s: %v\n", cfg.Name, err)
		os.Exit(1)
	}
}
