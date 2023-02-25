package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/tilt-dev/ctlptl/pkg/cluster"
)

func NewDockerDesktopCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "docker-desktop",
		Short: "Debugging tool for the Docker Desktop client",
		Example: "  ctlptl docker-desktop settings\n" +
			"  ctlptl docker-desktop set KEY VALUE",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "settings",
		Short: "Print the docker-desktop settings",
		Run:   withDockerDesktopClient("docker-desktop-settings", dockerDesktopSettings),
		Args:  cobra.ExactArgs(0),
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reset-cluster",
		Short: "Reset the docker-desktop Kubernetes cluster",
		Run:   withDockerDesktopClient("docker-desktop-reset-cluster", dockerDesktopResetCluster),
		Args:  cobra.ExactArgs(0),
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "open",
		Short: "Open docker-desktop",
		Run:   withDockerDesktopClient("docker-desktop-open", dockerDesktopOpen),
		Args:  cobra.ExactArgs(0),
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "quit",
		Short: "Shutdown docker-desktop",
		Run:   withDockerDesktopClient("docker-desktop-quit", dockerDesktopQuit),
		Args:  cobra.ExactArgs(0),
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set the docker-desktop settings",
		Long: "Set the docker-desktop settings\n\n" +
			"The first argument is the full path to the setting.\n\n" +
			"The second argument is the desired value.\n\n" +
			"Most settings are scalars. vm.fileSharing is a list of paths separated by commas.",
		Example: "  ctlptl docker-desktop set vm.resources.cpus 2\n" +
			"   ctlptl docker-desktop set kubernetes.enabled false\n" +
			"  ctlptl docker-desktop set vm.fileSharing /Users,/Volumes,/private,/tmp",
		Run:  withDockerDesktopClient("docker-desktop-set", dockerDesktopSet),
		Args: cobra.ExactArgs(2),
	})

	return cmd
}

func withDockerDesktopClient(name string, run func(client cluster.DockerDesktopClient, args []string) error) func(_ *cobra.Command, args []string) {
	return func(_ *cobra.Command, args []string) {
		a, err := newAnalytics()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "analytics: %v\n", err)
			os.Exit(1)
		}
		a.Incr(fmt.Sprintf("cmd.%s", name), nil)
		defer a.Flush(time.Second)

		c, err := cluster.NewDockerDesktopClient()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ctlptl docker-desktop: %v\n", err)
			os.Exit(1)
		}

		err = run(c, args)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ctlptl docker-desktop: %v\n", err)
			os.Exit(1)
		}
	}
}

func dockerDesktopSettings(c cluster.DockerDesktopClient, args []string) error {
	settings, err := c.SettingsValues(context.Background())
	if err != nil {
		return err
	}

	encoder := yaml.NewEncoder(os.Stdout)
	return encoder.Encode(settings)
}

func dockerDesktopSet(c cluster.DockerDesktopClient, args []string) error {
	return c.SetSettingValue(context.Background(), args[0], args[1])
}

func dockerDesktopResetCluster(c cluster.DockerDesktopClient, args []string) error {
	return c.ResetCluster(context.Background())
}

func dockerDesktopOpen(c cluster.DockerDesktopClient, args []string) error {
	return c.Open(context.Background())
}

func dockerDesktopQuit(c cluster.DockerDesktopClient, args []string) error {
	return c.Quit(context.Background())
}
