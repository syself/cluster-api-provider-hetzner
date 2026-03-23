package main

import (
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/syself/cluster-api-provider-hetzner/data"
)

func newRootCommand() *cobra.Command {
	ctrlLog.SetLogger(logr.Discard())
	data.RegisterEmbeddedInstallImageTGZ()

	rootCmd := &cobra.Command{
		Use:           "caphcli",
		Short:         "CAPH developer and operations CLI",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	rootCmd.AddCommand(newCheckBMServersCommand())

	return rootCmd
}
