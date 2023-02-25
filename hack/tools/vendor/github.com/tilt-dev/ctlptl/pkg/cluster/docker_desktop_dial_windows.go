//go:build windows
// +build windows

package cluster

import (
	"net"
	"os"
	"time"

	"gopkg.in/natefinch/npipe.v2"
)

func dockerDesktopBackendNativeSocketPaths() ([]string, error) {
	return []string{
		`\\.\pipe\dockerBackendNativeApiServer`,
		`\\.\pipe\dockerWebApiServer`,
	}, nil
}

// Use npipe.Dial to create a connection.
//
// npipe.Dial will wait if the socket doesn't exist. Stat it first and
// dial on a timeout.
//
// https://github.com/natefinch/npipe#func-dial
func dialDockerDesktop(socketPath string) (net.Conn, error) {
	_, err := os.Stat(socketPath)
	if err != nil {
		return nil, err
	}
	return npipe.DialTimeout(socketPath, 2*time.Second)
}

func dialDockerBackend() (net.Conn, error) {
	return dialDockerDesktop(`\\.\pipe\dockerBackendApiServer`)
}
