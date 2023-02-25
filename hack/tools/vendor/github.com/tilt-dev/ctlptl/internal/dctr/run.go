package dctr

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Docker Container client.
type Client interface {
	DaemonHost() string
	ImagePull(ctx context.Context, image string, options types.ImagePullOptions) (io.ReadCloser, error)

	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
}

func NewAPIClient(streams genericclioptions.IOStreams) (client.APIClient, error) {
	dockerCli, err := command.NewDockerCli(
		command.WithOutputStream(streams.Out),
		command.WithErrorStream(streams.ErrOut))
	if err != nil {
		return nil, fmt.Errorf("failed to create new docker API: %v", err)
	}

	newClientOpts := flags.NewClientOptions()
	flagset := pflag.NewFlagSet("docker", pflag.ContinueOnError)
	newClientOpts.Common.InstallFlags(flagset)
	newClientOpts.Common.SetDefaultOptions(flagset)

	err = dockerCli.Initialize(newClientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize docker API: %w", err)
	}
	return dockerCli.Client(), nil
}

// A simplified remove-container-if-necessary helper.
func RemoveIfNecessary(ctx context.Context, c Client, name string) error {
	container, err := c.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		return err
	}
	if container.ContainerJSONBase == nil {
		return nil
	}

	return c.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
		Force: true,
	})
}

// A simplified run-container-and-detach helper for background support containers (like socat and the registry).
func Run(ctx context.Context, c Client, name string, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig) error {

	ctr, err := c.ContainerInspect(ctx, name)
	if err == nil && (ctr.ContainerJSONBase != nil && ctr.State.Running) {
		// The service is already running!
		return nil
	} else if err == nil {
		// The service exists, but is not running
		err := c.ContainerRemove(ctx, name, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return fmt.Errorf("creating %s: %v", name, err)
		}
	} else if !client.IsErrNotFound(err) {
		return fmt.Errorf("inspecting %s: %v", name, err)
	}

	resp, err := c.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, name)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return fmt.Errorf("creating %s: %v", name, err)
		}

		err := pull(ctx, c, config.Image)
		if err != nil {
			return fmt.Errorf("pulling image %s: %v", config.Image, err)
		}

		resp, err = c.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, name)
		if err != nil {
			return fmt.Errorf("creating %s: %v", name, err)
		}
	}

	id := resp.ID
	err = c.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("starting %s: %v", name, err)
	}
	return nil
}

func pull(ctx context.Context, c Client, image string) error {
	resp, err := c.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %v", image, err)
	}
	defer resp.Close()

	_, _ = io.Copy(io.Discard, resp)
	return nil
}
