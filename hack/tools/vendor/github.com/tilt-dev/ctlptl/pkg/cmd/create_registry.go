package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/registry"
)

type CreateRegistryOptions struct {
	*genericclioptions.PrintFlags
	genericclioptions.IOStreams

	Registry *api.Registry
}

func NewCreateRegistryOptions() *CreateRegistryOptions {
	o := &CreateRegistryOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created"),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
		Registry: &api.Registry{
			TypeMeta: registry.TypeMeta(),
		},
	}
	return o
}

func (o *CreateRegistryOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "registry [name]",
		Short: "Create a registry with the given name",
		Example: "  ctlptl create registry ctlptl-registry\n" +
			"  ctlptl create registry ctlptl-registry --port=5000\n" +
			"  ctlptl create registry ctlptl-registry --port=5000 --listen-address 0.0.0.0",
		Run:  o.Run,
		Args: cobra.ExactArgs(1),
	}

	cmd.SetOut(o.Out)
	cmd.SetErr(o.ErrOut)
	o.PrintFlags.AddFlags(cmd)
	cmd.Flags().IntVar(&o.Registry.Port, "port", o.Registry.Port,
		"The port to expose the registry on host. If not specified, chooses a random port")
	cmd.Flags().StringVar(&o.Registry.ListenAddress, "listen-address", o.Registry.ListenAddress,
		"The host's IP address to bind the container to. If not set defaults to 127.0.0.1")
	cmd.Flags().StringVar(&o.Registry.Image, "image", registry.DefaultRegistryImageRef,
		"Registry image to use")

	return cmd
}

func (o *CreateRegistryOptions) Run(cmd *cobra.Command, args []string) {
	controller, err := registry.DefaultController(o.IOStreams)
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}

	err = o.run(controller, args[0])
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}
}

type registryCreator interface {
	Apply(ctx context.Context, registry *api.Registry) (*api.Registry, error)
	Get(ctx context.Context, name string) (*api.Registry, error)
}

func (o *CreateRegistryOptions) run(controller registryCreator, name string) error {
	a, err := newAnalytics()
	if err != nil {
		return err
	}
	a.Incr("cmd.create.registry", nil)
	defer a.Flush(time.Second)

	o.Registry.Name = name
	registry.FillDefaults(o.Registry)

	ctx := context.Background()
	_, err = controller.Get(ctx, o.Registry.Name)
	if err == nil {
		return fmt.Errorf("Cannot create registry: already exists")
	} else if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Cannot check registry: %v", err)
	}

	applied, err := controller.Apply(ctx, o.Registry)
	if err != nil {
		return err
	}

	printer, err := toPrinter(o.PrintFlags)
	if err != nil {
		return err
	}

	return printer.PrintObj(applied, o.Out)
}
