package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tilt-dev/wmclient/pkg/analytics"
)

func NewRootCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "ctlptl [command]",
		Short: "Mess around with local Kubernetes clusters without consequences",
		Example: "  ctlptl get clusters\n" +
			"  ctlptl apply -f my-cluster.yaml",
	}

	rootCmd.AddCommand(NewCreateOptions().Command())
	rootCmd.AddCommand(NewGetOptions().Command())
	rootCmd.AddCommand(NewApplyOptions().Command())
	rootCmd.AddCommand(NewDeleteOptions().Command())
	rootCmd.AddCommand(NewDockerDesktopCommand())
	rootCmd.AddCommand(newDocsCommand(rootCmd))
	rootCmd.AddCommand(analytics.NewCommand())
	rootCmd.AddCommand(NewSocatCommand())

	return rootCmd
}
