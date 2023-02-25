package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/pkg/errors"
	"github.com/tilt-dev/localregistry-go"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	cexec "github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
)

// minikube v1.26 completely changes the api for changing the registry
var v1_26 = semver.MustParse("1.26.0")
var v1_27 = semver.MustParse("1.27.0")

// minikubeAdmin uses the minikube CLI to manipulate a minikube cluster,
// once the underlying machine has been setup.
type minikubeAdmin struct {
	iostreams    genericclioptions.IOStreams
	runner       cexec.CmdRunner
	dockerClient dockerClient
}

func newMinikubeAdmin(iostreams genericclioptions.IOStreams, dockerClient dockerClient, runner cexec.CmdRunner) *minikubeAdmin {
	return &minikubeAdmin{
		iostreams:    iostreams,
		dockerClient: dockerClient,
		runner:       runner,
	}
}

func (a *minikubeAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("minikube")
	if err != nil {
		return fmt.Errorf("minikube not installed. Please install minikube with these instructions: https://minikube.sigs.k8s.io/")
	}
	return nil
}

type minikubeVersionResponse struct {
	MinikubeVersion string `json:"minikubeVersion"`
}

func (a *minikubeAdmin) version(ctx context.Context) (semver.Version, error) {
	out := bytes.NewBuffer(nil)
	err := a.runner.RunIO(ctx,
		genericclioptions.IOStreams{Out: out, ErrOut: a.iostreams.ErrOut},
		"minikube", "version", "-o", "json")
	if err != nil {
		return semver.Version{}, fmt.Errorf("minikube version: %v", err)
	}

	decoder := json.NewDecoder(out)
	response := minikubeVersionResponse{}
	err = decoder.Decode(&response)
	if err != nil {
		return semver.Version{}, fmt.Errorf("minikube version: %v", err)
	}
	v := response.MinikubeVersion
	if v == "" {
		return semver.Version{}, fmt.Errorf("minikube version not found")
	}
	result, err := semver.ParseTolerant(v)
	if err != nil {
		return semver.Version{}, fmt.Errorf("minikube version: %v", err)
	}
	return result, nil
}

func (a *minikubeAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	v, err := a.version(ctx)
	if err != nil {
		return err
	}
	isRegistryApiBroken := v.GTE(v1_26) && v.LT(v1_27)
	isRegistryApiV2 := v.GTE(v1_26)

	clusterName := desired.Name
	if registry != nil {
		// Assume the network name is the same as the cluster name,
		// which is true in minikube 0.15+. It's OK if it doesn't,
		// because we double-check if the registry is in the network.
		err := a.ensureRegistryDisconnected(ctx, registry, container.NetworkMode(clusterName))
		if err != nil {
			return err
		}
	}

	containerRuntime := "containerd"
	if desired.Minikube != nil && desired.Minikube.ContainerRuntime != "" {
		containerRuntime = desired.Minikube.ContainerRuntime
	}

	extraConfigs := []string{"kubelet.max-pods=500"}
	if desired.Minikube != nil && len(desired.Minikube.ExtraConfigs) > 0 {
		extraConfigs = desired.Minikube.ExtraConfigs
	}

	args := []string{
		"start",
	}

	if desired.Minikube != nil {
		args = append(args, desired.Minikube.StartFlags...)
	}

	args = append(args,
		"-p", clusterName,
		"--driver=docker",
		fmt.Sprintf("--container-runtime=%s", containerRuntime),
	)

	for _, c := range extraConfigs {
		args = append(args, fmt.Sprintf("--extra-config=%s", c))
	}

	if desired.MinCPUs != 0 {
		args = append(args, fmt.Sprintf("--cpus=%d", desired.MinCPUs))
	}
	if desired.KubernetesVersion != "" {
		args = append(args, "--kubernetes-version", desired.KubernetesVersion)
	}

	// https://github.com/tilt-dev/ctlptl/issues/239
	if registry != nil {
		if isRegistryApiBroken {
			return fmt.Errorf(
				"Error: Local registries are broken in minikube v1.26.\n" +
					"See: https://github.com/kubernetes/minikube/issues/14480 .\n" +
					"Please upgrade to minikube v1.27.")
		}
		args = append(args, "--insecure-registry", fmt.Sprintf("%s:%d", registry.Name, registry.Status.ContainerPort))
	}

	in := strings.NewReader("")

	err = a.runner.RunIO(ctx,
		genericclioptions.IOStreams{In: in, Out: a.iostreams.Out, ErrOut: a.iostreams.ErrOut},
		"minikube", args...)
	if err != nil {
		return errors.Wrap(err, "creating minikube cluster")
	}

	if registry != nil {
		container, err := a.dockerClient.ContainerInspect(ctx, clusterName)
		if err != nil {
			return errors.Wrap(err, "inspecting minikube cluster")
		}
		networkMode := container.HostConfig.NetworkMode
		err = a.ensureRegistryConnected(ctx, registry, networkMode)
		if err != nil {
			return err
		}

		if isRegistryApiV2 {
			err = a.applyContainerdPatchRegistryApiV2(ctx, desired, registry, networkMode)
			if err != nil {
				return err
			}
		} else {
			err = a.applyContainerdPatchRegistryApiV1(ctx, desired, registry, networkMode)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Minikube v0.15.0+ creates a unique network for each minikube cluster.
func (a *minikubeAdmin) ensureRegistryConnected(ctx context.Context, registry *api.Registry, networkMode container.NetworkMode) error {
	if networkMode.IsUserDefined() && !a.inRegistryNetwork(registry, networkMode) {
		err := a.dockerClient.NetworkConnect(ctx, networkMode.UserDefined(), registry.Name, nil)
		if err != nil {
			return errors.Wrap(err, "connecting registry")
		}
	}
	return nil
}

// Minikube hard-codes IP addresses in the cluster network.
// So make sure the registry is disconnected from the network before running
// "minikube start".
//
// https://github.com/tilt-dev/ctlptl/issues/144
func (a *minikubeAdmin) ensureRegistryDisconnected(ctx context.Context, registry *api.Registry, networkMode container.NetworkMode) error {
	if networkMode.IsUserDefined() && a.inRegistryNetwork(registry, networkMode) {
		err := a.dockerClient.NetworkDisconnect(ctx, networkMode.UserDefined(), registry.Name, false)
		if err != nil {
			return errors.Wrap(err, "disconnecting registry")
		}

		// Remove the network from the current set of networks attached to the registry. This allows the registry to be reconnected after
		// a cluster delete and create operation without removing the registry
		networks := []string{}
		for _, n := range registry.Status.Networks {
			if n != networkMode.UserDefined() {
				networks = append(networks, n)
			}
		}
		registry.Status.Networks = networks
	}
	return nil
}

// We want to make sure that the image is pullable from either:
// localhost:[registry-port] or
// [registry-name]:5000
// by cloning the registry config created by minikube's --insecure-registry.
func (a *minikubeAdmin) applyContainerdPatchRegistryApiV2(ctx context.Context, desired *api.Cluster, registry *api.Registry, networkMode container.NetworkMode) error {
	nodeOutput := bytes.NewBuffer(nil)
	err := a.runner.RunIO(ctx,
		genericclioptions.IOStreams{Out: nodeOutput, ErrOut: a.iostreams.ErrOut},
		"minikube", "-p", desired.Name, "node", "list")
	if err != nil {
		return errors.Wrap(err, "configuring minikube registry")
	}

	nodes := []string{}
	nodeOutputSplit := strings.Split(nodeOutput.String(), "\n")
	for _, line := range nodeOutputSplit {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		node := strings.TrimSpace(fields[0])
		if node == "" {
			continue
		}
		nodes = append(nodes, node)
	}

	for _, node := range nodes {
		networkHost := registry.Status.IPAddress
		if networkMode.IsUserDefined() {
			networkHost = registry.Name
		}

		err := a.runner.RunIO(ctx,
			a.iostreams,
			"minikube", "-p", desired.Name, "--node", node,
			"ssh", "sudo", "cp", `\-R`,
			fmt.Sprintf(`/etc/containerd/certs.d/%s\:%d`, networkHost, registry.Status.ContainerPort),
			fmt.Sprintf(`/etc/containerd/certs.d/localhost\:%d`, registry.Status.HostPort))
		if err != nil {
			return errors.Wrap(err, "configuring minikube registry")
		}

		err = a.runner.RunIO(ctx, a.iostreams, "minikube", "-p", desired.Name, "--node", node,
			"ssh", "sudo", "systemctl", "restart", "containerd")
		if err != nil {
			return errors.Wrap(err, "configuring minikube registry")
		}
	}
	return nil
}

// We still patch containerd so that the user can push/pull from localhost.
// But note that this will NOT survive across minikube stop and start.
// See https://github.com/tilt-dev/ctlptl/issues/180
func (a *minikubeAdmin) applyContainerdPatchRegistryApiV1(ctx context.Context, desired *api.Cluster, registry *api.Registry, networkMode container.NetworkMode) error {
	configPath := "/etc/containerd/config.toml"

	nodeOutput := bytes.NewBuffer(nil)
	err := a.runner.RunIO(ctx,
		genericclioptions.IOStreams{Out: nodeOutput, ErrOut: a.iostreams.ErrOut},
		"minikube", "-p", desired.Name, "node", "list")
	if err != nil {
		return errors.Wrap(err, "configuring minikube registry")
	}

	nodes := []string{}
	nodeOutputSplit := strings.Split(nodeOutput.String(), "\n")
	for _, line := range nodeOutputSplit {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		node := strings.TrimSpace(fields[0])
		if node == "" {
			continue
		}
		nodes = append(nodes, node)
	}

	for _, node := range nodes {
		networkHost := registry.Status.IPAddress
		if networkMode.IsUserDefined() {
			networkHost = registry.Name
		}

		// this is the most annoying sed expression i've ever had to write
		// minikube does not give us great primitives for writing files on the host machine :\
		// so we have to hack around the shell escaping on its interactive shell
		err := a.runner.RunIO(ctx,
			a.iostreams,
			"minikube", "-p", desired.Name, "--node", node,
			"ssh", "sudo", "sed", `\-i`,
			fmt.Sprintf(
				`s,\\\[plugins.\\\(\\\"\\\?.*cri\\\"\\\?\\\).registry.mirrors\\\],[plugins.\\\1.registry.mirrors]\\\n`+
					`\ \ \ \ \ \ \ \ [plugins.\\\1.registry.mirrors.\\\"localhost:%d\\\"]\\\n`+
					`\ \ \ \ \ \ \ \ \ \ endpoint\ =\ [\\\"http://%s:%d\\\"],`,
				registry.Status.HostPort, networkHost, registry.Status.ContainerPort),
			configPath)
		if err != nil {
			return errors.Wrap(err, "configuring minikube registry")
		}

		err = a.runner.RunIO(ctx, a.iostreams, "minikube", "-p", desired.Name, "--node", node,
			"ssh", "sudo", "systemctl", "restart", "containerd")
		if err != nil {
			return errors.Wrap(err, "configuring minikube registry")
		}
	}
	return nil
}

func (a *minikubeAdmin) inRegistryNetwork(registry *api.Registry, networkMode container.NetworkMode) bool {
	for _, n := range registry.Status.Networks {
		if n == networkMode.UserDefined() {
			return true
		}
	}
	return false
}

func (a *minikubeAdmin) LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error) {
	container, err := a.dockerClient.ContainerInspect(ctx, desired.Name)
	if err != nil {
		return nil, errors.Wrap(err, "inspecting minikube cluster")
	}
	networkMode := container.HostConfig.NetworkMode
	networkHost := registry.Status.IPAddress
	if networkMode.IsUserDefined() {
		networkHost = registry.Name
	}

	return &localregistry.LocalRegistryHostingV1{
		Host:                     fmt.Sprintf("localhost:%d", registry.Status.HostPort),
		HostFromClusterNetwork:   fmt.Sprintf("%s:%d", networkHost, registry.Status.ContainerPort),
		HostFromContainerRuntime: fmt.Sprintf("%s:%d", networkHost, registry.Status.ContainerPort),
		Help:                     "https://github.com/tilt-dev/ctlptl",
	}, nil
}

func (a *minikubeAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	err := a.runner.RunIO(ctx, a.iostreams, "minikube", "delete", "-p", config.Name)
	if err != nil {
		return errors.Wrap(err, "deleting minikube cluster")
	}
	return nil
}
