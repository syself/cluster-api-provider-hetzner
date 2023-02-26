package registry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"regexp"
	"reflect"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/phayes/freeport"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/dctr"
	"github.com/tilt-dev/ctlptl/internal/socat"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/docker"
)

var (
	typeMeta      = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Registry"}
	listTypeMeta  = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "RegistryList"}
	groupResource = schema.GroupResource{Group: "ctlptl.dev", Resource: "registries"}
)

const DefaultRegistryImageRef = "docker.io/library/registry:2" // The registry everyone uses.

// https://github.com/moby/moby/blob/v20.10.3/api/types/types.go#L313
const containerStateRunning = "running"

// ctlptlLabels are labels applied on create to registry containers.
//
// These are not considered for equality purposes, as ctlptl supports interop
// with local cluster tools that support self-managing a registry (e.g. k3d),
// so we don't want to unnecessarily re-create them. However, if ctlptl is used
// to modify (i.e. delete&create) a registry, the new object _will_ include
// these labels.
var ctlptlLabels = map[string]string{
	docker.ContainerLabelRole: "registry",
}

func TypeMeta() api.TypeMeta {
	return typeMeta
}

func ListTypeMeta() api.TypeMeta {
	return listTypeMeta
}

func FillDefaults(registry *api.Registry) {
	// Create a default name if one isn't in the YAML.
	// The default name is determined by the underlying product.
	if registry.Name == "" {
		registry.Name = "ctlptl-registry"
	}

	if registry.Image == "" {
		registry.Image = DefaultRegistryImageRef
	}
}

type socatController interface {
	ConnectRemoteDockerPort(ctx context.Context, port int) error
}

type Controller struct {
	iostreams    genericclioptions.IOStreams
	dockerClient dctr.Client
	socat        socatController
}

func NewController(iostreams genericclioptions.IOStreams, dockerClient dctr.Client) *Controller {
	return &Controller{
		iostreams:    iostreams,
		dockerClient: dockerClient,
		socat:        socat.NewController(dockerClient),
	}
}

func DefaultController(iostreams genericclioptions.IOStreams) (*Controller, error) {
	dockerClient, err := dctr.NewAPIClient(iostreams)
	if err != nil {
		return nil, err
	}

	return &Controller{
		iostreams:    iostreams,
		dockerClient: dockerClient,
		socat:        socat.NewController(dockerClient),
	}, nil
}

func (c *Controller) Get(ctx context.Context, name string) (*api.Registry, error) {
	list, err := c.List(ctx, ListOptions{FieldSelector: fmt.Sprintf("name=%s", name)})
	if err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		return nil, errors.NewNotFound(groupResource, name)
	}

	item := list.Items[0]
	return &item, nil
}

func (c *Controller) List(ctx context.Context, options ListOptions) (*api.RegistryList, error) {
	selector, err := fields.ParseSelector(options.FieldSelector)
	if err != nil {
		return nil, err
	}

	containers, err := c.registryContainers(ctx)
	if err != nil {
		return nil, err
	}

	result := []api.Registry{}
	for _, container := range containers {
		if len(container.Names) == 0 {
			continue
		}
		name := strings.TrimPrefix(container.Names[0], "/")
		created := time.Unix(container.Created, 0)

		inspect, err := c.dockerClient.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, err
		}
		env := inspect.Config.Env
		netSummary := container.NetworkSettings
		ipAddress := ""
		networks := []string{}
		if netSummary != nil {
			for network := range netSummary.Networks {
				networks = append(networks, network)
			}
			bridge, ok := netSummary.Networks["bridge"]
			if ok && bridge != nil {
				ipAddress = bridge.IPAddress
			}
		}
		sort.Strings(networks)

		listenAddress, hostPort, containerPort := c.ipAndPortsFrom(container.Ports)

		registry := &api.Registry{
			TypeMeta: typeMeta,
			Name:     name,
			Port:     hostPort,
			Status: api.RegistryStatus{
				CreationTimestamp: metav1.Time{Time: created},
				ContainerID:       container.ID,
				IPAddress:         ipAddress,
				HostPort:          hostPort,
				ListenAddress:     listenAddress,
				ContainerPort:     containerPort,
				Networks:          networks,
				State:             container.State,
				Labels:            container.Labels,
				Image:             container.Image,
				Env:               env,
			},
		}

		if !selector.Matches((*registryFields)(registry)) {
			continue
		}
		result = append(result, *registry)
	}
	return &api.RegistryList{
		TypeMeta: listTypeMeta,
		Items:    result,
	}, nil
}

func (c *Controller) ipAndPortsFrom(ports []types.Port) (listenAddress string, hostPort int, containerPort int) {
	for _, port := range ports {
		if port.PrivatePort == 5000 {
			return port.IP, int(port.PublicPort), int(port.PrivatePort)
		}
	}
	return "unknown", 0, 0
}

// Compare the desired registry against the existing registry, and reconcile
// the two to match.
func (c *Controller) Apply(ctx context.Context, desired *api.Registry) (*api.Registry, error) {
	FillDefaults(desired)
	existing, err := c.Get(ctx, desired.Name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if existing == nil {
		existing = &api.Registry{}
	}

	needsDelete := false
	if existing.Port != 0 && desired.Port != 0 && existing.Port != desired.Port {
		// If the port has changed, let's delete the registry and recreate it.
		needsDelete = true
	}
	if !imagesRefsEqual(existing.Status.Image, desired.Image) {
		needsDelete = true
	}
	if existing.Status.State != containerStateRunning {
		// If the registry has died, we need to recreate.
		needsDelete = true
	}
	for key, value := range desired.Labels {
		if existing.Status.Labels[key] != value {
			// If the user asked for a label that's not currently on
			// the container, the only way to add it is to re-create the whole container.
			needsDelete = true
		}
	}

	r := regexp.MustCompile("^(?P<key>[^=]+)=(?P<value>.*)")
	desiredEnvs := make(map[string]string)
	for _, value := range desired.Env {
		m := r.FindStringSubmatch(value)
		if m != nil {
			k := m[r.SubexpIndex("key")]
			v := m[r.SubexpIndex("value")]
			if k != "PATH" {
				desiredEnvs[k] = v
			}
		}
	}
	existingEnvs := make(map[string]string)
	for _, value := range existing.Status.Env {
		m := r.FindStringSubmatch(value)
		if m != nil {
			k := m[r.SubexpIndex("key")]
			v := m[r.SubexpIndex("value")]
			if k != "PATH" {
				existingEnvs[k] = v
			}
		}
	}
	if _, ok := desiredEnvs["REGISTRY_STORAGE_DELETE_ENABLED"]; ! ok {
		desiredEnvs["REGISTRY_STORAGE_DELETE_ENABLED"] = "true"
		desired.Env = append(desired.Env, "REGISTRY_STORAGE_DELETE_ENABLED=true")
	}
	if eq := reflect.DeepEqual(desiredEnvs, existingEnvs); ! eq {
		needsDelete = true
	}

	if needsDelete && existing.Name != "" {
		err = c.Delete(ctx, existing.Name)
		if err != nil {
			return nil, err
		}
		existing = existing.DeepCopy()
		existing.Status.ContainerID = ""
	}

	if existing.Status.ContainerID != "" {
		// If we got to this point, and the container id exists, then the registry is up to date!
		return existing, nil
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, "Creating registry %q...\n", desired.Name)

	err = dctr.RemoveIfNecessary(ctx, c.dockerClient, desired.Name)
	if err != nil {
		return nil, err
	}

	exposedPorts, portBindings, hostPort, err := c.portConfigs(existing, desired)
	if err != nil {
		return nil, err
	}

	err = dctr.Run(
		ctx,
		c.dockerClient,
		desired.Name,
		&container.Config{
			Hostname:     desired.Name,
			Image:        desired.Image,
			ExposedPorts: exposedPorts,
			Labels:       c.labelConfigs(existing, desired),
			Env:          desired.Env,
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "always"},
			PortBindings:  portBindings,
		},
		&network.NetworkingConfig{})
	if err != nil {
		return nil, err
	}

	err = c.maybeCreateForwarder(ctx, hostPort)
	if err != nil {
		return nil, err
	}

	return c.Get(ctx, desired.Name)
}

// Compute the ports to ContainerCreate() call
func (c *Controller) portConfigs(existing *api.Registry, desired *api.Registry) (map[nat.Port]struct{}, map[nat.Port][]nat.PortBinding, int, error) {
	// Preserve existing address by default
	hostPort := existing.Status.HostPort
	listenAddress := existing.Status.ListenAddress

	// Overwrite with desired behavior if specified.
	if desired.Port != 0 {
		hostPort = desired.Port
	}
	if desired.ListenAddress != "" {
		listenAddress = desired.ListenAddress
	}

	// Fill in defaults.
	if hostPort == 0 {
		freePort, err := freeport.GetFreePort()
		if err != nil {
			return nil, nil, 0, fmt.Errorf("creating registry: %v", err)
		}
		hostPort = freePort
	}

	if listenAddress == "" {
		// explicitly bind to IPv4 to prevent issues with the port forward when connected to a Docker network with IPv6 enabled
		// see https://github.com/docker/for-mac/issues/6015
		listenAddress = "127.0.0.1"
	}

	port := nat.Port("5000/tcp")
	portSet := map[nat.Port]struct{}{
		port: struct{}{},
	}
	portMap := map[nat.Port][]nat.PortBinding{
		port: []nat.PortBinding{
			{
				HostIP:   listenAddress,
				HostPort: fmt.Sprintf("%d", hostPort),
			},
		},
	}
	return portSet, portMap, hostPort, nil
}

// Compute the label configs to the container create call.
func (c *Controller) labelConfigs(existing *api.Registry, desired *api.Registry) map[string]string {
	newLabels := make(map[string]string, len(existing.Status.Labels)+len(desired.Labels)+len(ctlptlLabels))

	// Preserve existing labels.
	for k, v := range existing.Status.Labels {
		newLabels[k] = v
	}

	// Overwrite with new labels.
	for k, v := range desired.Labels {
		newLabels[k] = v
	}

	for k, v := range ctlptlLabels {
		newLabels[k] = v
	}

	return newLabels
}

func (c *Controller) maybeCreateForwarder(ctx context.Context, port int) error {
	if docker.IsLocalHost(c.dockerClient.DaemonHost()) {
		return nil
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut,
		" 🎮 Env DOCKER_HOST set. Assuming remote Docker and forwarding registry to localhost:%d\n", port)
	return c.socat.ConnectRemoteDockerPort(ctx, port)
}

func (c *Controller) registryContainers(ctx context.Context) ([]types.Container, error) {
	containers := make(map[string]types.Container)

	roleContainers, err := c.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("%s=registry", docker.ContainerLabelRole))),
		All: true,
	})
	if err != nil {
		return nil, err
	}
	for i := range roleContainers {
		containers[roleContainers[i].ID] = roleContainers[i]
	}

	ancestorContainers, err := c.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(
			filters.Arg("ancestor", DefaultRegistryImageRef)),
		All: true,
	})
	if err != nil {
		return nil, err
	}
	for i := range ancestorContainers {
		containers[ancestorContainers[i].ID] = ancestorContainers[i]
	}

	result := make([]types.Container, 0, len(containers))
	for _, c := range containers {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// Delete the given registry.
func (c *Controller) Delete(ctx context.Context, name string) error {
	registry, err := c.Get(ctx, name)
	if err != nil {
		return err
	}

	cID := registry.Status.ContainerID
	if cID == "" {
		return fmt.Errorf("container not running registry: %s", name)
	}

	return c.dockerClient.ContainerRemove(ctx, registry.Status.ContainerID, types.ContainerRemoveOptions{
		Force: true,
	})
}

// imageRefsEqual returns true of the normalized versions of the refs are equal.
//
// If the normalized versions are not equal OR either ref is invalid, false
// is returned.
func imagesRefsEqual(a, b string) bool {
	aRef, err := reference.ParseNormalizedNamed(a)
	if err != nil {
		return false
	}

	bRef, err := reference.ParseNormalizedNamed(b)
	if err != nil {
		return false
	}

	return aRef.String() == bRef.String()
}
