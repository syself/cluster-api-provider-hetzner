package cluster

import (
	"context"

	"github.com/tilt-dev/localregistry-go"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

// A cluster admin provides the basic start/stop functionality of a cluster,
// independent of the configuration of the machine it's running on.
type Admin interface {
	EnsureInstalled(ctx context.Context) error
	Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error

	// Infers the LocalRegistryHosting that this admin will try to configure.
	LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error)

	Delete(ctx context.Context, config *api.Cluster) error
}

// An extension of cluster admin that indicates the cluster configuration can be
// modified for use from inside containers.
type AdminInContainer interface {
	ModifyConfigInContainer(ctx context.Context, cluster *api.Cluster, containerID string, dockerClient dockerClient, configWriter configWriter) error
}
