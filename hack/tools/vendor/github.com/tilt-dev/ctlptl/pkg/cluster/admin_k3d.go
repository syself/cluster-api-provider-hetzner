package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"github.com/tilt-dev/localregistry-go"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	cexec "github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/api/k3dv1alpha4"
)

// Support for v1alpha4 format.
var v5_3 = semver.MustParse("5.3.0")

// k3dAdmin uses the k3d CLI to manipulate a k3d cluster,
// once the underlying machine has been setup.
type k3dAdmin struct {
	iostreams genericclioptions.IOStreams
	runner    cexec.CmdRunner
}

func newK3DAdmin(iostreams genericclioptions.IOStreams, runner cexec.CmdRunner) *k3dAdmin {
	return &k3dAdmin{
		iostreams: iostreams,
		runner:    runner,
	}
}

func (a *k3dAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("k3d")
	if err != nil {
		return fmt.Errorf("k3d not installed. Please install k3d with these instructions: https://k3d.io/#installation")
	}
	return nil
}

func (a *k3dAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	k3dV, err := a.version(ctx)
	if err != nil {
		return errors.Wrap(err, "detecting k3d version")
	}

	if desired.K3D != nil && desired.K3D.V1Alpha4Simple != nil && k3dV.LT(v5_3) {
		return fmt.Errorf("k3d v1alpha4 config file only supported on v5.3+")
	}

	// We generate a cluster config on all versions
	// because it does some useful validation.
	k3dConfig, err := a.clusterConfig(desired, registry)
	if err != nil {
		return errors.Wrap(err, "creating k3d cluster")
	}

	// Delete any orphaned cluster resources, ignoring any errors.
	// This can happen if the cluster exists but has been removed from the kubeconfig.
	_ = a.Delete(ctx, desired)

	if k3dV.LT(v5_3) {
		// 5.2 and below
		args := []string{"cluster", "create", k3dConfig.Name}
		if registry != nil {
			args = append(args, "--registry-use", registry.Name)
		}

		err := a.runner.RunIO(ctx,
			genericclioptions.IOStreams{Out: a.iostreams.Out, ErrOut: a.iostreams.ErrOut},
			"k3d", args...)
		if err != nil {
			return errors.Wrap(err, "creating k3d cluster")
		}

		return nil
	}

	// 5.3 and above.
	buf := bytes.NewBuffer(nil)
	encoder := yaml.NewEncoder(buf)
	err = encoder.Encode(k3dConfig)
	if err != nil {
		return errors.Wrap(err, "creating k3d cluster")
	}

	args := []string{"cluster", "create", k3dConfig.Name, "--config", "-"}
	err = a.runner.RunIO(ctx,
		genericclioptions.IOStreams{In: buf, Out: a.iostreams.Out, ErrOut: a.iostreams.ErrOut},
		"k3d", args...)
	if err != nil {
		return errors.Wrap(err, "creating k3d cluster")
	}

	return nil
}

// K3D manages the LocalRegistryHosting config itself :cheers:
func (a *k3dAdmin) LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error) {
	return nil, nil
}

func (a *k3dAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	clusterName := config.Name
	if !strings.HasPrefix(clusterName, "k3d-") {
		return fmt.Errorf("all k3d clusters must have a name with the prefix k3d-*")
	}

	k3dName := strings.TrimPrefix(clusterName, "k3d-")
	err := a.runner.RunIO(ctx,
		a.iostreams,
		"k3d", "cluster", "delete", k3dName)
	if err != nil {
		return errors.Wrap(err, "deleting k3d cluster")
	}
	return nil
}

func (a *k3dAdmin) version(ctx context.Context) (semver.Version, error) {
	out := bytes.NewBuffer(nil)
	err := a.runner.RunIO(ctx,
		genericclioptions.IOStreams{Out: out, ErrOut: a.iostreams.ErrOut},
		"k3d", "version")
	if err != nil {
		return semver.Version{}, fmt.Errorf("k3d version: %v", err)
	}

	v := strings.TrimPrefix(strings.Split(out.String(), "\n")[0], "k3d version ")
	result, err := semver.ParseTolerant(v)
	if err != nil {
		return semver.Version{}, fmt.Errorf("k3d version: %v", err)
	}
	return result, nil
}

func (a *k3dAdmin) clusterConfig(desired *api.Cluster, registry *api.Registry) (*k3dv1alpha4.SimpleConfig, error) {
	var k3dConfig *k3dv1alpha4.SimpleConfig
	if desired.K3D == nil || desired.K3D.V1Alpha4Simple == nil {
		k3dConfig = &k3dv1alpha4.SimpleConfig{}
	} else {
		k3dConfig = desired.K3D.V1Alpha4Simple.DeepCopy()
	}
	k3dConfig.Kind = "Simple"
	k3dConfig.APIVersion = "k3d.io/v1alpha4"

	clusterName := desired.Name
	if !strings.HasPrefix(clusterName, "k3d-") {
		return nil, fmt.Errorf("all k3d clusters must have a name with the prefix k3d-*")
	}

	k3dConfig.Name = strings.TrimPrefix(clusterName, "k3d-")
	if registry != nil {
		k3dConfig.Registries.Use = append(k3dConfig.Registries.Use, registry.Name)
	}
	return k3dConfig, nil
}
