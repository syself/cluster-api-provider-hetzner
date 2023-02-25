//go:build !windows
// +build !windows

package cluster

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"

	"github.com/mitchellh/go-homedir"
)

func dockerDesktopBackendNativeSocketPaths() ([]string, error) {
	socketDir, err := dockerDesktopSocketDir()
	if err != nil {
		return nil, err
	}

	return []string{
		// Newer versions of docker desktop use this socket.
		filepath.Join(socketDir, "backend.native.sock"),

		// Older versions of docker desktop use this socket.
		filepath.Join(socketDir, "gui-api.sock"),
	}, nil
}

func dialDockerDesktop(socketPath string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("Cannot dial docker-desktop on %s", runtime.GOOS)
	}

	return net.Dial("unix", socketPath)
}

func dialDockerBackend() (net.Conn, error) {
	socketDir, err := dockerDesktopSocketDir()
	if err != nil {
		return nil, err
	}
	return dialDockerDesktop(filepath.Join(socketDir, "backend.sock"))
}

func dockerDesktopSocketDir() (string, error) {
	homedir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homedir, "Library/Containers/com.docker.docker/Data"), nil
	case "linux":
		return filepath.Join(homedir, ".docker/desktop"), nil
	}
	return "", fmt.Errorf("Cannot find docker desktop directory on %s", runtime.GOOS)
}
