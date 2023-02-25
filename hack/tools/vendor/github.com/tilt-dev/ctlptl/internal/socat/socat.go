// Manage socat network routers for remote docker instances.
package socat

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/tilt-dev/ctlptl/internal/dctr"
)

const serviceName = "ctlptl-portforward-service"

type Controller struct {
	client dctr.Client
}

func NewController(client dctr.Client) *Controller {
	return &Controller{client: client}
}

// Connect a port on the local machine to a port on a remote docker machine.
func (c *Controller) ConnectRemoteDockerPort(ctx context.Context, port int) error {
	err := c.StartRemotePortforwarder(ctx)
	if err != nil {
		return err
	}
	return c.StartLocalPortforwarder(ctx, port)
}

// Create a port-forwarding server on the same machine that's running
// Docker. This server accepts connections and routes them to localhost ports
// on the same machine.
func (c *Controller) StartRemotePortforwarder(ctx context.Context) error {
	return dctr.Run(
		ctx,
		c.client,
		serviceName,
		&container.Config{
			Hostname:   serviceName,
			Image:      "alpine/socat",
			Entrypoint: []string{"/bin/sh"},
			Cmd:        []string{"-c", "while true; do sleep 1000; done"},
		},
		&container.HostConfig{
			NetworkMode:   "host",
			RestartPolicy: container.RestartPolicy{Name: "always"},
		},
		&network.NetworkingConfig{})
}

// Returns the socat process listening on a port, plus its commandline.
func (c *Controller) socatProcessOnPort(port int) (*process.Process, string, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, "", err
	}
	for _, p := range processes {
		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}
		if strings.HasPrefix(cmdline, fmt.Sprintf("socat TCP-LISTEN:%d,", port)) {
			return p, cmdline, nil
		}
	}
	return nil, "", nil
}

// Create a port-forwarding server on the local machine, forwarding connections
// to the same port on the remote Docker server.
func (c *Controller) StartLocalPortforwarder(ctx context.Context, port int) error {
	args := []string{
		fmt.Sprintf("TCP-LISTEN:%d,reuseaddr,fork", port),
		fmt.Sprintf("EXEC:'docker exec -i %s socat STDIO TCP:localhost:%d'", serviceName, port),
	}

	existing, cmdline, err := c.socatProcessOnPort(port)
	if err != nil {
		return fmt.Errorf("start portforwarder: %v", err)
	}

	if existing != nil {
		expectedCmdline := strings.Join(append([]string{"socat"}, args...), " ")
		if expectedCmdline == cmdline {
			// Already running.
			return nil
		}

		// Kill and restart.
		err := existing.KillWithContext(ctx)
		if err != nil {
			return fmt.Errorf("start portforwarder: %v", err)
		}
	}

	cmd := exec.Command("socat", args...)
	err = cmd.Start()
	if err != nil {
		_, err := exec.LookPath("socat")
		if err != nil {
			return fmt.Errorf("socat not installed: ctlptl requires 'socat' to be installed when setting up clusters on a remote Docker daemon")
		}

		return fmt.Errorf("creating local portforwarder: %v", err)
	}

	for i := 0; i < 100; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for local portforwarder")
}
