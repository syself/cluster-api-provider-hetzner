package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioncheck"
)

func newCheckBMServersCommand() *cobra.Command {
	cfg := provisioncheck.DefaultConfig()
	cfg.Input = os.Stdin
	cfg.Output = os.Stdout

	cmd := &cobra.Command{
		Use:   "check-bm-servers",
		Short: "Validate rescue and provisioning reliability for one bare-metal server",
		Long: `Validate rescue and provisioning reliability for one HetznerBareMetalHost from a local YAML file.

The command does not talk to Kubernetes. It reads one local YAML file containing
HetznerBareMetalHost objects and then talks directly to Hetzner Robot plus the
target server.`,
		Example: `  caphcli check-bm-servers \
    --file test/e2e/data/infrastructure-hetzner/v1beta1/bases/hetznerbaremetalhosts.yaml \
    --name bm-e2e-1731561`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.HbmhYAMLFile == "" {
				return errors.New("--file is required")
			}

			if _, err := os.Stat(cfg.HbmhYAMLFile); err != nil {
				return fmt.Errorf("check --file: %w", err)
			}

			if err := provisioncheck.Run(context.Background(), cfg); err != nil {
				return fmt.Errorf("caphcli check-bm-servers failed for %q: %w", cfg.Name, err)
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&cfg.HbmhYAMLFile, "file", "", "Path to a local YAML file containing HetznerBareMetalHost objects (required)")
	flags.StringVar(&cfg.Name, "name", "", "HetznerBareMetalHost metadata.name. Optional if YAML contains exactly one host")
	flags.StringVar(&cfg.ImagePath, "image-path", provisioncheck.DefaultUbuntu2404ImagePath, "Installimage IMAGE path for operating system inside the Hetzner rescue system")
	flags.BoolVar(&cfg.Force, "force", false, "Skip the destructive-action confirmation prompt")
	flags.DurationVar(&cfg.PollInterval, "poll-interval", provisioncheck.DefaultPollInterval, "Polling interval for wait steps")
	flags.DurationVar(&cfg.Timeouts.LoadInput, "timeout-load-input", provisioncheck.DefaultLoadInputTimeout, "Timeout for input parsing + env loading")
	flags.DurationVar(&cfg.Timeouts.EnsureSSHKey, "timeout-ensure-ssh-key", provisioncheck.DefaultEnsureSSHKeyTimeout, "Timeout for ensuring SSH key in Robot")
	flags.DurationVar(&cfg.Timeouts.FetchServerDetails, "timeout-fetch-server", provisioncheck.DefaultFetchServerDetailsTimeout, "Timeout for fetching server details from Robot")
	flags.DurationVar(&cfg.Timeouts.ActivateRescue, "timeout-activate-rescue", provisioncheck.DefaultActivateRescueTimeout, "Timeout for activating rescue boot")
	flags.DurationVar(&cfg.Timeouts.RebootToRescue, "timeout-reboot-rescue", provisioncheck.DefaultRebootToRescueTimeout, "Timeout for requesting reboot to rescue")
	flags.DurationVar(&cfg.Timeouts.WaitForRescue, "timeout-wait-rescue", provisioncheck.DefaultWaitForRescueTimeout, "Timeout for waiting until rescue SSH is reachable")
	flags.DurationVar(&cfg.Timeouts.CheckDiskInRescue, "timeout-check-disk-rescue", provisioncheck.DefaultCheckDiskInRescueTimeout, "Timeout for checking target disks in rescue")
	flags.DurationVar(&cfg.Timeouts.InstallUbuntu, "timeout-install", provisioncheck.DefaultInstallUbuntuTimeout, "Timeout for one Ubuntu install step")
	flags.DurationVar(&cfg.Timeouts.RebootToOS, "timeout-reboot-os", provisioncheck.DefaultRebootToOSTimeout, "Timeout for rebooting into installed OS")
	flags.DurationVar(&cfg.Timeouts.WaitForOS, "timeout-wait-os", provisioncheck.DefaultWaitForOSTimeout, "Timeout for waiting until installed OS is reachable")

	return cmd
}
