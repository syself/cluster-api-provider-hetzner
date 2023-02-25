package docker

import (
	"strings"
)

const ContainerLabelRole = "dev.tilt.ctlptl.role"

// Checks whether the Docker daemon is running on a local machine.
// Remote docker daemons will likely need a port forwarder to work properly.
func IsLocalHost(dockerHost string) bool {
	return dockerHost == "" ||

		// Check all the "standard" docker localhosts.
		// https://github.com/docker/cli/blob/a32cd16160f1b41c1c4ae7bee4dac929d1484e59/opts/hosts.go#L22
		strings.HasPrefix(dockerHost, "tcp://localhost:") ||
		strings.HasPrefix(dockerHost, "tcp://127.0.0.1:") ||

		// https://github.com/moby/moby/blob/master/client/client_windows.go#L4
		strings.HasPrefix(dockerHost, "npipe:") ||

		// https://github.com/moby/moby/blob/master/client/client_unix.go#L6
		strings.HasPrefix(dockerHost, "unix:")
}

// Checks whether the DOCKER_HOST looks like a local Docker Engine.
func IsLocalDockerEngineHost(dockerHost string) bool {
	if strings.HasPrefix(dockerHost, "unix:") {
		// Many tools (like colima) try to masquerade as Docker Desktop but run
		// on a different socket.
		// see:
		// https://github.com/tilt-dev/ctlptl/issues/196
		// https://docs.docker.com/desktop/faqs/#how-do-i-connect-to-the-remote-docker-engine-api
		return strings.Contains(dockerHost, "/var/run/docker.sock") ||
			// Docker Desktop for Linux - socket is in ~/.docker/desktop/docker.sock
			strings.HasSuffix(dockerHost, "/.docker/desktop/docker.sock") ||
			// Docker Desktop for Mac 4.13+ - socket is in ~/.docker/run/docker.sock
			strings.HasSuffix(dockerHost, "/.docker/run/docker.sock")
	}

	// Docker daemons on other local protocols are treated as docker desktop.
	return IsLocalHost(dockerHost)
}

// Checks whether the DOCKER_HOST looks like a local Docker Desktop.
// A local Docker Engine has some additional APIs for VM management (i.e., Docker Desktop).
func IsLocalDockerDesktop(dockerHost string, os string) bool {
	if os == "darwin" || os == "windows" {
		return IsLocalDockerEngineHost(dockerHost)
	}
	return strings.HasPrefix(dockerHost, "unix:") &&
		strings.HasSuffix(dockerHost, "/.docker/desktop/docker.sock")
}
