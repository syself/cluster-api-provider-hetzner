package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"github.com/tilt-dev/ctlptl/pkg/registry"
	"github.com/tilt-dev/ctlptl/pkg/visitor"
)

type ApplyOptions struct {
	*genericclioptions.PrintFlags
	*genericclioptions.FileNameFlags
	genericclioptions.IOStreams

	Filenames []string
}

func NewApplyOptions() *ApplyOptions {
	o := &ApplyOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created"),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
	}
	o.FileNameFlags = &genericclioptions.FileNameFlags{Filenames: &o.Filenames}
	return o
}

func (o *ApplyOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "apply -f FILENAME",
		Short: "Apply a cluster config to the currently running clusters",
		Example: "  ctlptl apply -f cluster.yaml\n" +
			"  cat cluster.yaml | ctlptl apply -f -",
		Run: o.Run,
	}

	cmd.SetOut(o.Out)
	cmd.SetErr(o.ErrOut)
	o.FileNameFlags.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

func (o *ApplyOptions) Run(cmd *cobra.Command, args []string) {
	if len(o.Filenames) == 0 {
		fmt.Fprintf(o.ErrOut, "Expected source files with -f")
		os.Exit(1)
	}

	err := o.run()
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}
}

func (o *ApplyOptions) run() error {
	a, err := newAnalytics()
	if err != nil {
		return err
	}
	a.Incr("cmd.apply", nil)
	defer a.Flush(time.Second)

	ctx := context.TODO()

	printer, err := toPrinter(o.PrintFlags)
	if err != nil {
		return err
	}

	visitors, err := visitor.FromStrings(o.Filenames, o.In)
	if err != nil {
		return err
	}

	objects, err := visitor.DecodeAll(visitors)
	if err != nil {
		return err
	}

	var cc *cluster.Controller
	var rc *registry.Controller
	for _, obj := range objects {
		switch obj := obj.(type) {
		case *api.Registry:
			if rc == nil {
				rc, err = registry.DefaultController(o.IOStreams)
				if err != nil {
					return err
				}
			}

			newObj, err := rc.Apply(ctx, obj)
			if err != nil {
				return err
			}

			err = printer.PrintObj(newObj, o.Out)
			if err != nil {
				return err
			}
		}
	}

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *api.Cluster:
			if cc == nil {
				cc, err = cluster.DefaultController(o.IOStreams)
				if err != nil {
					return err
				}
			}

			newObj, err := cc.Apply(ctx, obj)
			if err != nil {
				return err
			}

			err = printer.PrintObj(newObj, o.Out)
			if err != nil {
				return err
			}

		case *api.Registry:
			// Handled above
			continue

		default:
			return fmt.Errorf("unrecognized type: %T", obj)
		}
	}
	return nil
}
