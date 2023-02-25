package cluster

import (
	"context"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stringid"

	"github.com/tilt-dev/ctlptl/internal/dctr"
)

type dockerClient interface {
	dctr.Client
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (types.Info, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error
}

type detectInContainer interface {
	insideContainer(ctx context.Context) string
}

// InsideContainer checks the current host and docker client to see if we are
// running inside a container with a Docker-out-of-Docker-mounted socket. It
// checks if:
//
//   - The effective DOCKER_HOST is `/var/run/docker.sock`
//   - The hostname looks like a container "short id" and is a valid, running
//     container
//
// Returns a non-empty string representing the container ID if inside a container.
func insideContainer(ctx context.Context, client dockerClient) string {
	// allows fake client to mock the result
	if detect, ok := client.(detectInContainer); ok {
		return detect.insideContainer(ctx)
	}

	if client.DaemonHost() != "unix:///var/run/docker.sock" {
		return ""
	}

	containerID, err := os.Hostname()
	if err != nil {
		return ""
	}

	if !stringid.IsShortID(containerID) {
		return ""
	}

	container, err := client.ContainerInspect(ctx, containerID)
	if err != nil {
		return ""
	}

	if !container.State.Running {
		return ""
	}

	return containerID
}
