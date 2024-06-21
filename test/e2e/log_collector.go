/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

const (
	hetznerPrivateKeyFilePath = "SSH_KEY_PATH"
)

type logCollector struct{}

// CollectMachinePoolLog implements the CollectMachinePoolLog method of the LogCollector interface.
func (collector logCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *expv1.MachinePool, _ string) error {
	return nil
}

// CollectInfrastructureLogs implements the CollectInfrastructureLog method of the LogCollector interface.
func (collector logCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}

// CollectMachineLog implements the CollectMachineLog method of the LogCollector interface.
func (collector logCollector) CollectMachineLog(_ context.Context, _ client.Client, m *clusterv1.Machine, outputPath string) error {
	var hostIPAddr string
	for _, address := range m.Status.Addresses {
		if address.Type != clusterv1.MachineExternalIP {
			continue
		}
		hostIPAddr = address.Address
		break
	}

	execToPathFn := func(hostFileName, command string, args ...string) func() error {
		return func() error {
			f, err := createOutputFile(filepath.Join(outputPath, hostFileName))
			if err != nil {
				return err
			}
			defer f.Close()
			return executeRemoteCommand(f, hostIPAddr, command, args...)
		}
	}

	copyDirFn := func(pathToDir, dirName string) func() error {
		return func() error {
			f, err := os.Create("/tmp/" + m.Name) // #nosec
			if err != nil {
				return err
			}

			tempfileName := f.Name()
			outputDir := filepath.Join(outputPath, dirName)

			defer os.Remove(tempfileName)

			if err := executeRemoteCommand(
				f, hostIPAddr,
				"tar", "--hard-dereference", "-hcf", "-", pathToDir); err != nil {
				return fmt.Errorf("failed to tar dir %s: %w", pathToDir, err)
			}

			err = os.MkdirAll(outputDir, 0o750)
			if err != nil {
				return err
			}

			cmd := exec.Command("tar", "--extract", "--file", tempfileName, "--directory", outputDir) // #nosec
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to run command: %w", err)
			}

			return nil
		}
	}

	return kinderrors.AggregateConcurrent([]func() error{
		execToPathFn("kern.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-k"),
		execToPathFn("kubelet.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-u", "kubelet.service"),
		execToPathFn("kubelet-version.txt",
			"kubelet", "--version"),
		execToPathFn("containerd.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-u", "containerd.service"),
		execToPathFn("cloud-init.log",
			"cat", "/var/log/cloud-init.log"),
		execToPathFn("cloud-init-output.log",
			"cat", "/var/log/cloud-init-output.log"),
		copyDirFn("/var/log/pods", "pods"),
		copyDirFn("/etc/kubernetes", "kubernetes"),
	})
}

func createOutputFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return os.Create(path) // #nosec
}

func executeRemoteCommand(f io.StringWriter, hostIPAddr, command string, args ...string) error {
	config, err := newSSHConfig()
	if err != nil {
		return err
	}
	port := "22"

	hostClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", hostIPAddr, port), config)
	if err != nil {
		return fmt.Errorf("dialing host IP address at %s: %w", hostIPAddr, err)
	}
	defer hostClient.Close()

	session, err := hostClient.NewSession()
	if err != nil {
		return fmt.Errorf("opening SSH session: %w", err)
	}
	defer session.Close()

	// Run the command and write the captured stdout to the file
	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}
	if err = session.Run(command); err != nil {
		return fmt.Errorf("running command %q: %w", command, err)
	}
	if _, err = f.WriteString(stdoutBuf.String()); err != nil {
		return fmt.Errorf("writing output to file: %w", err)
	}

	return nil
}

// newSSHConfig returns a configuration to use for SSH connections to remote machines.
func newSSHConfig() (*ssh.ClientConfig, error) {
	sshPrivateKeyContent, err := readPrivateKey()
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(sshPrivateKeyContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key %s: %w", sshPrivateKeyContent, err)
	}

	config := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return config, nil
}

func readPrivateKey() ([]byte, error) {
	privateKeyFilePath := os.Getenv(hetznerPrivateKeyFilePath)
	if privateKeyFilePath == "" {
		return nil, fmt.Errorf("private key information missing. Please set %s environment variable", hetznerPrivateKeyFilePath)
	}

	return os.ReadFile(privateKeyFilePath) // #nosec
}
