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
	"encoding/base64"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

const (
	// HetznerPrivateKeyContent is the env var "HETZNER_SSH_PRIV" with base64-encoded private key content.
	HetznerPrivateKeyContent = "HETZNER_SSH_PRIV"
)

type logCollector struct{}

// CollectMachineLogByExternalIP provides a minimal entrypoint for ad-hoc log collection.
func CollectMachineLogByExternalIP(ctx context.Context, machineName, externalIP, outputPath string) error {
	if machineName == "" {
		return fmt.Errorf("machine name must not be empty")
	}
	if externalIP == "" {
		return fmt.Errorf("external IP must not be empty")
	}
	if outputPath == "" {
		return fmt.Errorf("output path must not be empty")
	}

	m := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: machineName,
		},
		Status: clusterv1.MachineStatus{
			Addresses: []clusterv1.MachineAddress{
				{
					Type:    clusterv1.MachineExternalIP,
					Address: externalIP,
				},
			},
		},
	}

	return logCollector{}.CollectMachineLog(ctx, nil, m, outputPath)
}

// CollectMachinePoolLog implements the CollectMachinePoolLog method of the LogCollector interface.
func (collector logCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *expv1.MachinePool, _ string) error {
	return nil
}

// CollectInfrastructureLogs implements the CollectInfrastructureLog method of the LogCollector interface.
func (collector logCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}

// CollectMachineLog implements the CollectMachineLog method of the LogCollector interface.
func (collector logCollector) CollectMachineLog(ctx context.Context, c client.Client, m *clusterv1.Machine, outputPath string) error {
	hostIPAddr, err := machineExternalIP(ctx, c, m)
	if err != nil {
		return fmt.Errorf("CollectMachineLog failed: %w", err)
	}

	execToPathFn := func(hostFileName, command string, args ...string) func() error {
		return func() error {
			f, err := createOutputFile(filepath.Join(outputPath, hostFileName))
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			return executeRemoteCommand(f, hostIPAddr, command, args...)
		}
	}

	copyDirFn := func(pathToDir, dirName string) func() error {
		return func() error {
			f, err := os.CreateTemp("/tmp", m.Name+"-*.tar") // #nosec
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			tempfileName := f.Name()
			outputDir := filepath.Join(outputPath, dirName)

			defer func() { _ = os.Remove(tempfileName) }()

			if err := executeRemoteCommandStdoutOnly(
				f, hostIPAddr,
				"tar", "--hard-dereference", "-hcf", "-", pathToDir); err != nil {
				return fmt.Errorf("failed to tar dir %s: %w", pathToDir, err)
			}

			err = os.MkdirAll(outputDir, 0o750)
			if err != nil {
				return err
			}

			cmd := exec.CommandContext(context.Background(), "tar", "--extract", "--file", tempfileName, "--directory", outputDir) // #nosec
			output, err := cmd.CombinedOutput()
			if err != nil {
				outputForError := formatOutputForError(string(output))
				if outputForError != "" {
					return fmt.Errorf("failed to extract dir %s: %w; output: %s", pathToDir, err, outputForError)
				}
				return fmt.Errorf("failed to extract dir %s: %w", pathToDir, err)
			}

			return nil
		}
	}

	return kinderrors.AggregateConcurrent([]func() error{
		execToPathFn("kern.log",
			"journalctl", "--no-pager", "--output=short-precise", "-k"),
		execToPathFn("kubelet.log",
			"journalctl", "--no-pager", "--output=short-precise", "-u", "kubelet.service"),
		execToPathFn("kubelet-version.txt",
			"kubelet", "--version"),
		execToPathFn("containerd.log",
			"journalctl", "--no-pager", "--output=short-precise", "-u", "containerd.service"),
		execToPathFn("var-log-k8s-ls.log",
			"ls", "-lah", "/var/log/containers", "/var/log/pods"),
		execToPathFn("cloud-init.log",
			"cat", "/var/log/cloud-init.log"),
		execToPathFn("cloud-init-output.log",
			"cat", "/var/log/cloud-init-output.log"),
		copyDirFn("/var/log/pods", "pods"),
		copyDirFn("/etc/kubernetes", "kubernetes"),
	})
}

func machineExternalIP(ctx context.Context, c client.Client, m *clusterv1.Machine) (string, error) {
	if hostIPAddr := externalIPFromAddresses(m.Status.Addresses); hostIPAddr != "" {
		return hostIPAddr, nil
	}

	baseErr := fmt.Errorf("machine %q has no ExternalIP: machine.status.addresses: %+v", m.Name, m.Status.Addresses)
	if c == nil {
		return "", baseErr
	}

	hostIPAddr, err := infrastructureMachineExternalIP(ctx, c, m)
	if err != nil {
		stdlog.Printf(
			"CollectMachineLog: failed to resolve external IP via infrastructure machine for %s/%s (%s %s): %v",
			m.Namespace, m.Name, m.Spec.InfrastructureRef.Kind, m.Spec.InfrastructureRef.Name, err,
		)
		return "", fmt.Errorf("%w; infrastructure fallback failed: %w", baseErr, err)
	}

	if hostIPAddr != "" {
		return hostIPAddr, nil
	}

	return "", baseErr
}

func infrastructureMachineExternalIP(ctx context.Context, c client.Client, m *clusterv1.Machine) (string, error) {
	if c == nil {
		return "", fmt.Errorf("client is nil")
	}

	if m.Spec.InfrastructureRef.Name == "" {
		return "", fmt.Errorf("machine.spec.infrastructureRef.name is empty")
	}

	key := client.ObjectKey{
		Namespace: infrastructureRefNamespace(m.Spec.InfrastructureRef, m.Namespace),
		Name:      m.Spec.InfrastructureRef.Name,
	}

	switch m.Spec.InfrastructureRef.Kind {
	case "HetznerBareMetalMachine":
		hbmm := &infrav1.HetznerBareMetalMachine{}
		if err := c.Get(ctx, key, hbmm); err != nil {
			return "", fmt.Errorf("get HetznerBareMetalMachine %s: %w", key, err)
		}
		hostIPAddr := externalIPFromAddresses(hbmm.Status.Addresses)
		if hostIPAddr == "" {
			return "", fmt.Errorf("HetznerBareMetalMachine %s has no ExternalIP in status.addresses: %+v", key, hbmm.Status.Addresses)
		}
		return hostIPAddr, nil
	case "HCloudMachine":
		hm := &infrav1.HCloudMachine{}
		if err := c.Get(ctx, key, hm); err != nil {
			return "", fmt.Errorf("get HCloudMachine %s: %w", key, err)
		}
		hostIPAddr := externalIPFromAddresses(hm.Status.Addresses)
		if hostIPAddr == "" {
			return "", fmt.Errorf("HCloudMachine %s has no ExternalIP in status.addresses: %+v", key, hm.Status.Addresses)
		}
		return hostIPAddr, nil
	default:
		return "", fmt.Errorf("unsupported infrastructureRef kind %q", m.Spec.InfrastructureRef.Kind)
	}
}

func infrastructureRefNamespace(ref corev1.ObjectReference, defaultNamespace string) string {
	if ref.Namespace != "" {
		return ref.Namespace
	}
	return defaultNamespace
}

func externalIPFromAddresses(addresses []clusterv1.MachineAddress) string {
	for _, address := range addresses {
		if address.Type != clusterv1.MachineExternalIP {
			continue
		}
		return address.Address
	}
	return ""
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

	var hostClient *ssh.Client
	for attempt := 1; attempt <= 3; attempt++ {
		hostClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%s", hostIPAddr, port), config)
		if err != nil {
			if attempt < 3 && isRetryableSSHError(err) {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return fmt.Errorf("dialing host IP address at %s: %w", hostIPAddr, err)
		}
		break
	}
	defer func() { _ = hostClient.Close() }()

	session, err := hostClient.NewSession()
	if err != nil {
		return fmt.Errorf("opening SSH session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Capture both stdout and stderr to keep useful failure details.
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var combinedOutputBuf bytes.Buffer
	session.Stdout = io.MultiWriter(&combinedOutputBuf, &stdoutBuf)
	session.Stderr = io.MultiWriter(&combinedOutputBuf, &stderrBuf)
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}
	if err = session.Run(command); err != nil {
		output := combinedOutputBuf.String()
		if _, writeErr := f.WriteString(output); writeErr != nil {
			return fmt.Errorf("running command %q: %w (writing output to file: %v)", command, err, writeErr)
		}
		stdoutForError := formatOutputForError(stdoutBuf.String())
		stderrForError := formatOutputForError(stderrBuf.String())
		if stdoutForError != "" || stderrForError != "" {
			return fmt.Errorf("running command %q: %w; stdout: %q; stderr: %q", command, err, stdoutForError, stderrForError)
		}
		return fmt.Errorf("running command %q: %w", command, err)
	}
	if _, err = f.WriteString(combinedOutputBuf.String()); err != nil {
		return fmt.Errorf("writing output to file: %w", err)
	}

	return nil
}

func executeRemoteCommandStdoutOnly(w io.Writer, hostIPAddr, command string, args ...string) error {
	config, err := newSSHConfig()
	if err != nil {
		return err
	}
	port := "22"

	var hostClient *ssh.Client
	for attempt := 1; attempt <= 3; attempt++ {
		hostClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%s", hostIPAddr, port), config)
		if err != nil {
			if attempt < 3 && isRetryableSSHError(err) {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return fmt.Errorf("dialing host IP address at %s: %w", hostIPAddr, err)
		}
		break
	}
	defer func() { _ = hostClient.Close() }()

	session, err := hostClient.NewSession()
	if err != nil {
		return fmt.Errorf("opening SSH session: %w", err)
	}
	defer func() { _ = session.Close() }()

	var stderrBuf bytes.Buffer
	session.Stdout = w
	session.Stderr = &stderrBuf
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}
	if err = session.Run(command); err != nil {
		stderrForError := formatOutputForError(stderrBuf.String())
		if stderrForError != "" {
			return fmt.Errorf("running command %q: %w; stderr: %q", command, err, stderrForError)
		}
		return fmt.Errorf("running command %q: %w", command, err)
	}

	return nil
}

func isRetryableSSHError(err error) bool {
	message := strings.ToLower(err.Error())

	return strings.Contains(message, "handshake failed") ||
		strings.Contains(message, "connection reset by peer") ||
		strings.Contains(message, "connection refused") ||
		strings.Contains(message, "i/o timeout")
}

func formatOutputForError(output string) string {
	const maxOutputLength = 3000

	formatted := strings.TrimSpace(strings.ToValidUTF8(output, "?"))
	if formatted == "" {
		return ""
	}

	if len(formatted) > maxOutputLength {
		return formatted[:maxOutputLength] + "..."
	}

	return formatted
}

// newSSHConfig returns a configuration to use for SSH connections to remote machines.
func newSSHConfig() (*ssh.ClientConfig, error) {
	sshPrivateKeyContent, err := readPrivateKey()
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(sshPrivateKeyContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key from %s: %w", HetznerPrivateKeyContent, err)
	}

	config := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return config, nil
}

func readPrivateKey() ([]byte, error) {
	privateKeyBase64 := os.Getenv(HetznerPrivateKeyContent)
	if privateKeyBase64 == "" {
		return nil, fmt.Errorf("private key information missing. Please set %s environment variable with base64-encoded private key content", HetznerPrivateKeyContent)
	}
	privateKeyContent, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to base64-decode %s: %w", HetznerPrivateKeyContent, err)
	}
	return privateKeyContent, nil
}
