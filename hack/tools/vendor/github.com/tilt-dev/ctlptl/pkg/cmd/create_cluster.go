package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/clusterid"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
)

type CreateClusterOptions struct {
	*genericclioptions.PrintFlags
	genericclioptions.IOStreams

	Cluster *api.Cluster
}

func NewCreateClusterOptions() *CreateClusterOptions {
	o := &CreateClusterOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created"),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
		Cluster: &api.Cluster{
			TypeMeta: cluster.TypeMeta(),
			Minikube: &api.MinikubeCluster{},
		},
	}
	return o
}

func (o *CreateClusterOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "cluster [product]",
		Short: "Create a cluster with the given local Kubernetes product",
		Example: "  ctlptl create cluster docker-desktop\n" +
			"  ctlptl create cluster kind --registry=ctlptl-registry",
		Run:  o.Run,
		Args: cobra.ExactArgs(1),
	}

	cmd.SetOut(o.Out)
	cmd.SetErr(o.ErrOut)
	o.PrintFlags.AddFlags(cmd)
	cmd.Flags().StringVar(&o.Cluster.Registry, "registry",
		o.Cluster.Registry, "Connect the cluster to the named registry")
	cmd.Flags().StringVar(&o.Cluster.Name, "name",
		o.Cluster.Name, "Names the context. If not specified, uses the default cluster name for this Kubernetes product")
	cmd.Flags().IntVar(&o.Cluster.MinCPUs, "min-cpus",
		o.Cluster.MinCPUs, "Sets the minimum CPUs for the cluster")
	cmd.Flags().StringVar(&o.Cluster.KubernetesVersion, "kubernetes-version",
		o.Cluster.KubernetesVersion, "Sets the kubernetes version for the cluster, if possible")
	cmd.Flags().StringSliceVar(&o.Cluster.Minikube.StartFlags, "minikube-start-flags",
		o.Cluster.Minikube.StartFlags, "Minikube extra start flags (only applicable to a minikube cluster)")
	cmd.Flags().StringSliceVar(&o.Cluster.Minikube.ExtraConfigs, "minikube-extra-configs",
		o.Cluster.Minikube.ExtraConfigs, "Minikube extra configs (only applicable to a minikube cluster)")
	cmd.Flags().StringVar(&o.Cluster.Minikube.ContainerRuntime, "minikube-container-runtime",
		o.Cluster.Minikube.ContainerRuntime, "Minikube container runtime (only applicable to a minikube cluster)")

	return cmd
}

func (o *CreateClusterOptions) Run(cmd *cobra.Command, args []string) {
	controller, err := cluster.DefaultController(o.IOStreams)
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

type clusterCreator interface {
	Apply(ctx context.Context, cluster *api.Cluster) (*api.Cluster, error)
	Get(ctx context.Context, name string) (*api.Cluster, error)
}

func (o *CreateClusterOptions) run(controller clusterCreator, product string) error {
	a, err := newAnalytics()
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
		os.Exit(1)
	}
	a.Incr("cmd.create.cluster", nil)
	defer a.Flush(time.Second)

	o.Cluster.Product = product

	// Zero out the minikube config if not used.
	if product != string(clusterid.ProductMinikube) || cmp.Equal(o.Cluster.Minikube, &api.MinikubeCluster{}) {
		o.Cluster.Minikube = nil
	}

	cluster.FillDefaults(o.Cluster)

	ctx := context.Background()
	_, err = controller.Get(ctx, o.Cluster.Name)
	if err == nil {
		return fmt.Errorf("Cannot create cluster: already exists")
	} else if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Cannot check cluster: %v", err)
	}

	applied, err := controller.Apply(ctx, o.Cluster)
	if err != nil {
		return err
	}

	printer, err := toPrinter(o.PrintFlags)
	if err != nil {
		return err
	}

	return printer.PrintObj(applied, o.Out)
}
