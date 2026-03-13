// Command hbmh-provision-check validates rescue/provision reliability for one HBMH from YAML input.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	_ "github.com/syself/cluster-api-provider-hetzner/data"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioncheck"
)

func main() {
	ctrlLog.SetLogger(logr.Discard())

	// Recreate default flag set to avoid unrelated global flags from imported packages.
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	pflag.Usage = func() {
		out := pflag.CommandLine.Output()
		_, _ = fmt.Fprintf(out, "Usage: %s [flags]\n\n", os.Args[0])
		_, _ = fmt.Fprintln(out, "Validates rescue/provision reliability for one HetznerBareMetalHost from YAML input.")
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
		_, _ = fmt.Fprintln(out, "Flags:")
		pflag.PrintDefaults()
	}

	cfg := provisioncheck.DefaultConfig()

	pflag.StringVar(&cfg.YAMLFile, "file", "", "Path to a YAML file containing HetznerBareMetalHost objects (required)")
	pflag.StringVar(&cfg.Name, "name", "", "HetznerBareMetalHost metadata.name. Optional if YAML contains exactly one host")
	pflag.StringVar(&cfg.ImagePath, "image-path", cfg.ImagePath, "Installimage IMAGE path for Ubuntu 24.04")
	pflag.BoolVar(&cfg.Force, "force", cfg.Force, "Skip the destructive-action confirmation prompt")

	pflag.DurationVar(&cfg.PollInterval, "poll-interval", cfg.PollInterval, "Polling interval for wait steps")
	pflag.DurationVar(&cfg.Timeouts.LoadInput, "timeout-load-input", cfg.Timeouts.LoadInput, "Timeout for input parsing + env loading")
	pflag.DurationVar(&cfg.Timeouts.EnsureSSHKey, "timeout-ensure-ssh-key", cfg.Timeouts.EnsureSSHKey, "Timeout for ensuring SSH key in Robot")
	pflag.DurationVar(&cfg.Timeouts.FetchServerDetails, "timeout-fetch-server", cfg.Timeouts.FetchServerDetails, "Timeout for fetching server details from Robot")
	pflag.DurationVar(&cfg.Timeouts.ActivateRescue, "timeout-activate-rescue", cfg.Timeouts.ActivateRescue, "Timeout for activating rescue boot")
	pflag.DurationVar(&cfg.Timeouts.RebootToRescue, "timeout-reboot-rescue", cfg.Timeouts.RebootToRescue, "Timeout for requesting reboot to rescue")
	pflag.DurationVar(&cfg.Timeouts.WaitForRescue, "timeout-wait-rescue", cfg.Timeouts.WaitForRescue, "Timeout for waiting until rescue SSH is reachable")
	pflag.DurationVar(&cfg.Timeouts.InstallUbuntu, "timeout-install", cfg.Timeouts.InstallUbuntu, "Timeout for one Ubuntu install step")
	pflag.DurationVar(&cfg.Timeouts.RebootToOS, "timeout-reboot-os", cfg.Timeouts.RebootToOS, "Timeout for rebooting into installed OS")
	pflag.DurationVar(&cfg.Timeouts.WaitForOS, "timeout-wait-os", cfg.Timeouts.WaitForOS, "Timeout for waiting until installed OS is reachable")

	pflag.Parse()

	if cfg.YAMLFile == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		os.Exit(2)
	}

	resolved, err := provisioncheck.ResolveYAMLPath(cfg.YAMLFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve --file: %v\n", err)
		os.Exit(2)
	}
	cfg.YAMLFile = resolved

	if err := provisioncheck.Run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "hbmh-provision-check failed. %s: %v\n", cfg.Name, err)
		os.Exit(1)
	}
}
