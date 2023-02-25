package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/dctr"
	"github.com/tilt-dev/ctlptl/internal/socat"
)

func NewSocatCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "socat",
		Short: "Use socat to connect components. Experimental.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "connect-remote-docker",
		Short:   "Connects a local port to a remote port on a machine running Docker",
		Example: "  ctlptl socat connect-remote-docker [port]\n",
		Run:     connectRemoteDocker,
		Args:    cobra.ExactArgs(1),
	})

	return cmd
}

func connectRemoteDocker(cmd *cobra.Command, args []string) {
	port, err := strconv.Atoi(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect-remote-docker: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	dockerAPI, err := dctr.NewAPIClient(streams)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect-remote-docker: %v\n", err)
		os.Exit(1)
	}

	c := socat.NewController(dockerAPI)
	err = c.ConnectRemoteDockerPort(ctx, port)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "connect-remote-docker: %v\n", err)
		os.Exit(1)
	}
}
