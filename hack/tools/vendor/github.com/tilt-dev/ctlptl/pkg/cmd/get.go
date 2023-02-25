package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"github.com/tilt-dev/ctlptl/pkg/registry"
)

type GetOptions struct {
	*genericclioptions.PrintFlags
	genericclioptions.IOStreams
	StartTime      time.Time
	IgnoreNotFound bool
	FieldSelector  string
}

func NewGetOptions() *GetOptions {
	return &GetOptions{
		PrintFlags: genericclioptions.NewPrintFlags(""),
		IOStreams:  genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
		StartTime:  time.Now(),
	}
}

func (o *GetOptions) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "get [type] [name]",
		Short: "Read currently running clusters and registries",
		Long: `Read the status of currently running clusters and registries.

Supports the same flags as kubectl for selecting
and printing fields. The kubectl cheat sheet may help:

https://kubernetes.io/docs/reference/kubectl/cheatsheet/#formatting-output
`,
		Example: "  ctlptl get\n" +
			"  ctlptl get cluster microk8s -o yaml\n" +
			"  ctlptl get cluster kind-kind -o template --template '{{.status.localRegistryHosting.host}}'\n",
		Run:  o.Run,
		Args: cobra.MaximumNArgs(2),
	}

	cmd.SetOut(o.Out)
	cmd.SetErr(o.ErrOut)
	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")

	return cmd
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) {
	a, err := newAnalytics()
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "analytics: %v\n", err)
		os.Exit(1)
	}
	a.Incr("cmd.get", nil)
	defer a.Flush(time.Second)

	ctx := context.TODO()
	t := "cluster"
	if len(args) >= 1 {
		t = args[0]
	}
	var resource runtime.Object
	switch t {
	case "registry", "registries":
		c, err := registry.DefaultController(o.IOStreams)
		if err != nil {
			_, _ = fmt.Fprintf(o.ErrOut, "Loading controller: %v\n", err)
			os.Exit(1)
		}

		if len(args) >= 2 {
			resource, err = c.Get(ctx, args[1])
			if err != nil {
				if errors.IsNotFound(err) && o.IgnoreNotFound {
					os.Exit(0)
				}
				_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
				os.Exit(1)
			}
		} else {
			resource, err = c.List(ctx, registry.ListOptions{FieldSelector: o.FieldSelector})
			if err != nil {
				_, _ = fmt.Fprintf(o.ErrOut, "List registries: %v\n", err)
				os.Exit(1)
			}
		}

	case "cluster", "clusters":
		c, err := cluster.DefaultController(o.IOStreams)
		if err != nil {
			_, _ = fmt.Fprintf(o.ErrOut, "Loading controller: %v\n", err)
			os.Exit(1)
		}

		if len(args) >= 2 {
			resource, err = normalizedGet(ctx, c, args[1])
			if err != nil {
				if errors.IsNotFound(err) && o.IgnoreNotFound {
					os.Exit(0)
				}
				_, _ = fmt.Fprintf(o.ErrOut, "%v\n", err)
				os.Exit(1)
			}
		} else {
			resource, err = c.List(ctx, cluster.ListOptions{FieldSelector: o.FieldSelector})
			if err != nil {
				_, _ = fmt.Fprintf(o.ErrOut, "List clusters: %v\n", err)
				os.Exit(1)
			}
		}

	default:
		_, _ = fmt.Fprintf(o.ErrOut, "Unrecognized type: %s. Possible values: cluster, registry.\n", t)
		os.Exit(1)
	}

	err = o.Print(resource)
	if err != nil {
		_, _ = fmt.Fprintf(o.ErrOut, "Error: %s\n", err)
		os.Exit(1)
	}
}

func (o *GetOptions) ToPrinter() (printers.ResourcePrinter, error) {
	if !o.OutputFlagSpecified() {
		return printers.NewTablePrinter(printers.PrintOptions{}), nil
	}
	return toPrinter(o.PrintFlags)
}

func (o *GetOptions) Print(obj runtime.Object) error {
	if obj == nil {
		fmt.Println("No resources found")
		return nil
	}

	printer, err := o.ToPrinter()
	if err != nil {
		return err
	}

	err = printer.PrintObj(o.transformForOutput(obj), o.Out)
	if err != nil {
		return err
	}
	return nil
}

func (o *GetOptions) OutputFlagSpecified() bool {
	return o.PrintFlags.OutputFlagSpecified != nil && o.PrintFlags.OutputFlagSpecified()
}

func (o *GetOptions) transformForOutput(obj runtime.Object) runtime.Object {
	if o.OutputFlagSpecified() {
		return obj
	}

	switch r := obj.(type) {
	case *api.Registry:
		return o.registriesAsTable([]api.Registry{*r})
	case *api.RegistryList:
		return o.registriesAsTable(r.Items)
	case *api.Cluster:
		return o.clustersAsTable([]api.Cluster{*r})
	case *api.ClusterList:
		return o.clustersAsTable(r.Items)
	default:
		return obj
	}
}

func (o *GetOptions) clustersAsTable(clusters []api.Cluster) runtime.Object {
	table := metav1.Table{
		TypeMeta: metav1.TypeMeta{Kind: "Table", APIVersion: "metav1.k8s.io"},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			metav1.TableColumnDefinition{
				Name: "Current",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Name",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Product",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Age",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Registry",
				Type: "string",
			},
		},
	}

	for _, cluster := range clusters {
		age := "unknown"
		cTime := cluster.Status.CreationTimestamp.Time
		if !cTime.IsZero() {
			age = duration.ShortHumanDuration(o.StartTime.Sub(cTime))
		}

		rHost := ""
		if cluster.Status.LocalRegistryHosting != nil {
			rHost = cluster.Status.LocalRegistryHosting.Host
		}
		if rHost == "" {
			rHost = "none"
		}

		current := ""
		if cluster.Status.Current {
			current = "*"
		}

		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []interface{}{
				current,
				cluster.Name,
				cluster.Product,
				age,
				rHost,
			},
		})
	}

	return &table
}

func (o *GetOptions) registriesAsTable(registries []api.Registry) runtime.Object {
	table := metav1.Table{
		TypeMeta: metav1.TypeMeta{Kind: "Table", APIVersion: "metav1.k8s.io"},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			metav1.TableColumnDefinition{
				Name: "Name",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Host Address",
				Type: "int",
			},
			metav1.TableColumnDefinition{
				Name: "Container Address",
				Type: "string",
			},
			metav1.TableColumnDefinition{
				Name: "Age",
				Type: "string",
			},
		},
	}

	// sort chronologically newest -> oldest to match `docker ps` behavior
	sort.SliceStable(registries, func(i, j int) bool {
		return registries[i].Status.CreationTimestamp.After(registries[j].Status.CreationTimestamp.Time)
	})

	for _, registry := range registries {
		age := "unknown"
		cTime := registry.Status.CreationTimestamp.Time
		if !cTime.IsZero() {
			age = duration.ShortHumanDuration(o.StartTime.Sub(cTime))
		}

		hostAddress := "none"
		if registry.Status.HostPort != 0 {
			hostAddress = fmt.Sprintf("%s:%d", registry.Status.ListenAddress, registry.Status.HostPort)
		}

		containerAddress := "none"
		if registry.Status.ContainerPort != 0 && registry.Status.IPAddress != "" {
			containerAddress = fmt.Sprintf("%s:%d", registry.Status.IPAddress, registry.Status.ContainerPort)
		}

		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []interface{}{
				registry.Name,
				hostAddress,
				containerAddress,
				age,
			},
		})
	}

	return &table
}
