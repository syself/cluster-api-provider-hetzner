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

// Package sshclient contains the interface to speak to bare metal servers with ssh.
package sshclient

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	sshTimeOut time.Duration = 5 * time.Second
)

//go:embed detect-linux-on-another-disk.sh
var detectLinuxOnAnotherDiskShellScript string

//go:embed wipe-disk.sh
var wipeDiskShellScript string

//go:embed check-disk.sh
var checkDiskShellScript string

//go:embed nic-info.sh
var nicInfoShellScript string

var downloadFromOciShellScript = `#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This scripts gets copied from the controller into the rescue system
# of the bare-metal machine.

set -euo pipefail

image="${1:-}"
outfile="${2:-}"

function usage {
    echo "$0 image outfile."
    echo "  Download a machine image from a container registry"
    echo "  image: for example ghcr.io/foo/bar/my-machine-image:v9"
    echo "  outfile: Created file. Usually with file extensions '.tgz'"
    echo "  If the oci registry needs a token, then the script uses OCI_REGISTRY_AUTH_TOKEN (if set)"
    echo "  Example 1: of OCI_REGISTRY_AUTH_TOKEN: mygithubuser:mypassword"
    echo "  Example 2: of OCI_REGISTRY_AUTH_TOKEN: ghp_SN51...."
    echo
}
if [ -z "$outfile" ]; then
    usage
    exit 1
fi
OCI_REGISTRY_AUTH_TOKEN="${OCI_REGISTRY_AUTH_TOKEN:-}" # github:$GITHUB_TOKEN

# Extract registry
registry="${image%%/*}"

# Extract scope and tag
remainder="${image#*/}"
scope="${remainder%:*}"
tag="${remainder##*:}"

if [[ -z "$registry" || -z "$scope" || -z "$tag" ]]; then
    echo "failed to parse registry, scope and tag from image"
    echo "image=$image"
    echo "registry=$registry"
    echo "scope=$scope"
    echo "tag=$tag"
    exit 1
fi

function download_with_token {
    echo "download with token (OCI_REGISTRY_AUTH_TOKEN set)"
    if [[ "$OCI_REGISTRY_AUTH_TOKEN" != *:* ]]; then
        echo "Using OCI_REGISTRY_AUTH_TOKEN directly (no colon in token)"
        token=$(echo "$OCI_REGISTRY_AUTH_TOKEN" | base64)
    else
        echo "OCI_REGISTRY_AUTH_TOKEN contains colon. Doing login first"
        token=$(curl -fsSL -u "$OCI_REGISTRY_AUTH_TOKEN" "https://${registry}/token?scope=repository:$scope:pull" | jq -r '.token')
        if [ -z "$token" ]; then
            echo "Failed to get token for container registry"
            exit 1
        fi
        echo "Login to $registry was successful"
    fi
    digest=$(curl -sSL -H "Authorization: Bearer $token" -H "Accept: application/vnd.oci.image.manifest.v1+json" \
        "https://${registry}/v2/${scope}/manifests/${tag}" | jq -r '.layers[0].digest')

    if [ -z "$digest" ]; then
        echo "Failed to get digest from container registry"
        exit 1
    fi

    echo "Start download of $image"
    curl -fsSL -H "Authorization: Bearer $token" \
        "https://${registry}/v2/${scope}/blobs/$digest" >"$outfile"
}

function download_without_token {
    echo "download without token (OCI_REGISTRY_AUTH_TOKEN empty)"
    digest=$(curl -sSL -H "Accept: application/vnd.oci.image.manifest.v1+json" \
        "https://${registry}/v2/${scope}/manifests/${tag}" | jq -r '.layers[0].digest')

    if [ -z "$digest" ]; then
        echo "Failed to get digest from container registry"
        exit 1
    fi

    echo "Start download of $image"
    curl -fsSL "https://${registry}/v2/${scope}/blobs/$digest" >"$outfile"
}

if [ -z "$OCI_REGISTRY_AUTH_TOKEN" ]; then
    download_without_token
else
    download_with_token
fi
`

var (
	// ErrCommandExitedWithoutExitSignal means the ssh command exited unplanned.
	ErrCommandExitedWithoutExitSignal = errors.New("wait: remote command exited without exit status or exit signal")
	// ErrCommandExitedWithStatusOne means the ssh command exited with sttatus 1.
	ErrCommandExitedWithStatusOne = errors.New("Process exited with status 1") //nolint:stylecheck // this is used to check ssh errors

	// ErrConnectionRefused means the ssh connection was refused.
	ErrConnectionRefused = errors.New("connect: connection refused")
	// ErrAuthenticationFailed means ssh was unable to authenticate.
	ErrAuthenticationFailed = errors.New("ssh: unable to authenticate")
	// ErrEmptyStdOut means that StdOut equals empty string.
	ErrEmptyStdOut = errors.New("unexpected empty output in stdout")
	// ErrTimeout means that there is a timeout error.
	ErrTimeout = errors.New("i/o timeout")
	// ErrCheckDiskBrokenDisk means that a disk seams broken.
	ErrCheckDiskBrokenDisk = errors.New("CheckDisk failed")
	errSSHDialFailed       = errors.New("failed to dial ssh")
)

// Input defines an SSH input.
type Input struct {
	IP         string
	PrivateKey string
	Port       int
}

// Output defines the SSH output.
type Output struct {
	StdOut string
	StdErr string
	Err    error
}

// InstallImageState defines three states of the process.
type InstallImageState string

const (
	// InstallImageStateNotStartedYet means the process has not started yet.
	InstallImageStateNotStartedYet InstallImageState = "not-started-yet"
	// InstallImageStateRunning means the process is still running.
	InstallImageStateRunning InstallImageState = "running"
	// InstallImageStateFinished has finished.
	InstallImageStateFinished InstallImageState = "finished"
)

func (o Output) String() string {
	s := make([]string, 0, 3)
	stdout := strings.TrimSpace(o.StdOut)
	if stdout != "" {
		s = append(s, stdout)
	}
	stderr := strings.TrimSpace(o.StdErr)
	if stderr != "" {
		if len(s) > 0 {
			stderr = "Stderr: " + stderr
		}
		s = append(s, stderr)
	}
	if o.Err != nil {
		e := o.Err.Error()
		e = strings.TrimSpace(e)
		if len(s) > 0 {
			e = "Err: " + e
		}
		s = append(s, e)
	}
	return strings.Join(s, ". ")
}

// ExitStatus returns the exit status of the remote shell command.
// There are three case:
// First case: Remote command finished with exit 0: 0, nil.
// Second case: Remote command finished with non zero: N, nil.
// Third case: Remote command was not called successfully (like host not reachable): 0, err.
func (o Output) ExitStatus() (int, error) {
	var exitError *ssh.ExitError
	if errors.As(o.Err, &exitError) {
		return exitError.ExitStatus(), nil
	}
	return 0, o.Err
}

// Client is the interface defining all functions necessary to talk to a bare metal server via SSH.
type Client interface {
	GetHostName() Output
	GetHardwareDetailsRAM() Output
	GetHardwareDetailsNics() Output
	GetHardwareDetailsStorage() Output
	GetHardwareDetailsCPUArch() Output
	GetHardwareDetailsCPUModel() Output
	GetHardwareDetailsCPUClockGigahertz() Output
	GetHardwareDetailsCPUFlags() Output
	GetHardwareDetailsCPUThreads() Output
	GetHardwareDetailsCPUCores() Output
	GetHardwareDetailsDebug() Output
	GetInstallImageState() (InstallImageState, error)
	GetResultOfInstallImage() (string, error)
	GetCloudInitOutput() Output
	CreateAutoSetup(data string) Output
	DownloadImage(path, url string) Output
	CreatePostInstallScript(data string) Output
	ExecuteInstallImage(hasPostInstallScript bool) Output
	Reboot() Output
	CloudInitStatus() Output
	CheckCloudInitLogsForSigTerm() Output
	CleanCloudInitLogs() Output
	CleanCloudInitInstances() Output
	ResetKubeadm() Output
	UntarTGZ() Output
	DetectLinuxOnAnotherDisk(sliceOfWwns []string) Output

	// Erase filesystem, raid and partition-table signatures.
	// String "all" will wipe all disks.
	WipeDisk(ctx context.Context, sliceOfWwns []string) (string, error)

	// CheckDisk checks the given disks via smartctl.
	// ErrCheckDiskBrokenDisk gets returned, if a disk is broken.
	CheckDisk(ctx context.Context, sliceOfWwns []string) (info string, err error)

	// ExecutePreProvisionCommand executes a command before the provision process starts.
	// A non-zero exit status will indicate that provisioning should not start.
	ExecutePreProvisionCommand(ctx context.Context, preProvisionCommand string) (exitStatus int, stdoutAndStderr string, err error)
}

// Factory is the interface for creating new Client objects.
type Factory interface {
	NewClient(Input) Client
}

type sshFactory struct{}

// NewFactory creates a new factory for SSH clients.
func NewFactory() Factory {
	return &sshFactory{}
}

var _ = Factory(&sshFactory{})

// NewClient implements the NewClient method of the factory interface.
func (f *sshFactory) NewClient(in Input) Client {
	return &sshClient{
		privateSSHKey: in.PrivateKey,
		ip:            in.IP,
		port:          in.Port,
	}
}

type sshClient struct {
	ip            string
	privateSSHKey string
	port          int
}

var _ = Client(&sshClient{})

// GetHostName implements the GetHostName method of the SSHClient interface.
func (c *sshClient) GetHostName() Output {
	return c.runSSH("hostname")
}

// GetHardwareDetailsRAM implements the GetHardwareDetailsRAM method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsRAM() Output {
	return c.runSSH("grep MemTotal /proc/meminfo | awk '{print $2}'")
}

// GetHardwareDetailsNics implements the GetHardwareDetailsNics method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsNics() Output {
	return c.runSSH(fmt.Sprintf(`cat >/root/nic-info.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/nic-info.sh
/root/nic-info.sh
`, nicInfoShellScript))
}

// GetHardwareDetailsStorage implements the GetHardwareDetailsStorage method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsStorage() Output {
	return c.runSSH(`lsblk -b -P -o "NAME,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,HCTL,ROTA"`)
}

// GetHardwareDetailsCPUArch implements the GetHardwareDetailsCPUArch method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUArch() Output {
	return c.runSSH(`lscpu | grep "Architecture:" | awk '{print $2}'`)
}

// GetHardwareDetailsCPUModel implements the GetHardwareDetailsCPUModel method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUModel() Output {
	return c.runSSH(`lscpu | grep "Model name:" | awk '{$1=$2=""; print $0}' | sed "s/^[ \t]*//"`)
}

// GetHardwareDetailsCPUClockGigahertz implements the GetHardwareDetailsCPUClockGigahertz method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUClockGigahertz() Output {
	return c.runSSH(`lscpu | grep "CPU max MHz:" |  awk '{printf "%.1f", $4/1000}'`)
}

// GetHardwareDetailsCPUFlags implements the GetHardwareDetailsCPUFlags method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUFlags() Output {
	return c.runSSH(`lscpu | grep "Flags:" |  awk '{ $1=""; print $0}' | sed "s/^[ \t]*//"`)
}

// GetCloudInitOutput implements the GetCloudInitOutput method of the SSHClient interface.
func (c *sshClient) GetCloudInitOutput() Output {
	out := c.runSSH(`cat /var/log/cloud-init-output.log`)
	if out.Err == nil {
		out.StdOut = removeUselessLinesFromCloudInitOutput(out.StdOut)
	}
	return out
}

// GetHardwareDetailsCPUThreads implements the GetHardwareDetailsCPUThreads method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUThreads() Output {
	return c.runSSH(`lscpu | grep "CPU(s):" | head -1 |  awk '{ print $2}'`)
}

// GetHardwareDetailsCPUCores implements the GetHardwareDetailsCPUCores method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUCores() Output {
	return c.runSSH(`grep 'cpu cores' /proc/cpuinfo | uniq | awk '{print $4}'`)
}

// GetHardwareDetailsDebug implements the GetHardwareDetailsDebug method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsDebug() Output {
	return c.runSSH(`ip a; echo ==========----------==========;
	ethtool "*"; echo ==========----------==========;
	lspci; echo ==========----------==========;
	`)
}

// CreateAutoSetup implements the CreateAutoSetup method of the SSHClient interface.
func (c *sshClient) CreateAutoSetup(data string) Output {
	return c.runSSH(fmt.Sprintf(`cat << 'EOF_VIA_SSH' > /autosetup
%s
EOF_VIA_SSH`, data))
}

// DownloadImage implements the DownloadImage method of the SSHClient interface.
func (c *sshClient) DownloadImage(path, url string) Output {
	if !strings.HasPrefix(url, "oci://") {
		return c.runSSH(fmt.Sprintf(`curl -sLo "%q" "%q"`, path, url))
	}
	return c.runSSH(fmt.Sprintf(`cat << 'ENDOFSCRIPT' > /root/download-from-oci.sh
%s
ENDOFSCRIPT
chmod a+rx /root/download-from-oci.sh
OCI_REGISTRY_AUTH_TOKEN=%s /root/download-from-oci.sh %s %s`, downloadFromOciShellScript,
		os.Getenv("OCI_REGISTRY_AUTH_TOKEN"),
		strings.TrimPrefix(url, "oci://"), path))
}

// CreatePostInstallScript implements the CreatePostInstallScript method of the SSHClient interface.
func (c *sshClient) CreatePostInstallScript(data string) Output {
	out := c.runSSH(fmt.Sprintf(`cat << 'EOF_VIA_SSH' > /root/post-install.sh
%s
EOF_VIA_SSH`, data))

	if out.Err != nil || out.StdErr != "" {
		return out
	}
	return c.runSSH(`chmod +x /root/post-install.sh`)
}

// GetInstallImageState returns the running installimage processes.
func (c *sshClient) GetInstallImageState() (InstallImageState, error) {
	out := c.runSSH(`ps aux| grep installimage | grep -v grep; true`)
	if out.Err != nil {
		return "", fmt.Errorf("failed to run `ps aux` to get running installimage process: %w", out.Err)
	}
	if out.StdOut != "" {
		// installimage is running
		return InstallImageStateRunning, nil
	}

	out = c.runSSH(`[ -e /root/installimage-wrapper.sh.log ]`)
	exists, err := out.ExitStatus()
	if err != nil {
		return "", fmt.Errorf("failed to check if installimage-wrapper.sh.log exists: %w", err)
	}
	if exists == 0 {
		return InstallImageStateFinished, nil
	}
	return InstallImageStateNotStartedYet, nil
}

// ExecuteInstallImage implements the ExecuteInstallImage method of the SSHClient interface.
func (c *sshClient) ExecuteInstallImage(hasPostInstallScript bool) Output {
	var cmd string
	if hasPostInstallScript {
		cmd = `/root/hetzner-installimage/installimage -a -c /autosetup -x /root/post-install.sh`
	} else {
		cmd = `/root/hetzner-installimage/installimage -a -c /autosetup`
	}

	out := c.runSSH(fmt.Sprintf(`cat << 'EOF_VIA_SSH' > /root/installimage-wrapper.sh
#!/bin/bash
export TERM=xterm

# don't wait 20 seconds before starting: echo "x"
echo "x" | %s
EOF_VIA_SSH`, cmd))
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	out = c.runSSH(`chmod +x /root/installimage-wrapper.sh . `)
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	return c.runSSH(`nohup /root/installimage-wrapper.sh >/root/installimage-wrapper.sh.log 2>&1 </dev/null &`)
}

// GetResultOfInstallImage returns the logs of install-image.
// Before calling this method be sure that installimage is already terminated.
func (c *sshClient) GetResultOfInstallImage() (string, error) {
	out := c.runSSH(`cat /root/debug.txt`)
	if out.Err != nil {
		return "", fmt.Errorf("failed to get debug.txt: %w", out.Err)
	}
	debugTxt := out.StdOut

	out = c.runSSH(`cat /root/installimage-wrapper.sh.log`)
	if out.Err != nil {
		return "", fmt.Errorf("failed to get installimage-wrapper.sh.log: %w", out.Err)
	}
	wrapperLog := out.StdOut

	return fmt.Sprintf(`debug.txt:
%s

######################################

/root/installimage-wrapper.sh stdout+stderr:

%s
`,
		debugTxt, wrapperLog), nil
}

// Reboot implements the Reboot method of the SSHClient interface.
func (c *sshClient) Reboot() Output {
	out := c.runSSH(`reboot`)
	if out.Err != nil && strings.Contains(out.Err.Error(), ErrCommandExitedWithoutExitSignal.Error()) {
		return Output{}
	}
	return out
}

// CloudInitStatus implements the CloudInitStatus method of the SSHClient interface.
func (c *sshClient) CloudInitStatus() Output {
	out := c.runSSH("cloud-init status")
	if out.Err != nil && strings.Contains(out.Err.Error(), ErrCommandExitedWithStatusOne.Error()) {
		return Output{StdOut: "status: error"}
	}
	return out
}

// CheckCloudInitLogsForSigTerm implements the CheckCloudInitLogsForSigTerm method of the SSHClient interface.
func (c *sshClient) CheckCloudInitLogsForSigTerm() Output {
	out := c.runSSH(`cat /var/log/cloud-init.log | grep "SIGTERM"`)
	if out.Err != nil && strings.Contains(out.Err.Error(), ErrCommandExitedWithStatusOne.Error()) {
		return Output{}
	}
	return out
}

// CleanCloudInitLogs implements the CleanCloudInitLogs method of the SSHClient interface.
func (c *sshClient) CleanCloudInitLogs() Output {
	return c.runSSH(`cloud-init clean --logs`)
}

// CleanCloudInitInstances implements the CleanCloudInitInstances method of the SSHClient interface.
func (c *sshClient) CleanCloudInitInstances() Output {
	return c.runSSH(`rm -rf /var/lib/cloud/instances`)
}

// ResetKubeadm implements the ResetKubeadm method of the SSHClient interface.
func (c *sshClient) ResetKubeadm() Output {
	// if `kubeadm reset` fails, we disable all pods and related services explicitly.
	output := c.runSSH(`kubeadm reset -f 2>&1
	echo
	echo ========= stopping all pods =========
	crictl pods -q | while read -r podid; do
      crictl stopp "$podid"
    done
	echo
	echo ========= disabling kubelet =========
	systemctl disable --now kubelet
	echo
	echo ========= deleting directories =========
	rm -rf /etc/kubernetes /var/run/kubeadm /var/lib/etcd
	echo ========= done =========
`)
	return output
}

func (c *sshClient) DetectLinuxOnAnotherDisk(sliceOfWwns []string) Output {
	return c.runSSH(fmt.Sprintf(`cat >/root/detect-linux-on-another-disk.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/detect-linux-on-another-disk.sh
/root/detect-linux-on-another-disk.sh %s
`, detectLinuxOnAnotherDiskShellScript, strings.Join(sliceOfWwns, " ")))
}

var (
	// I found no details about the format. I found these examples:
	// 10:00:00:05:1e:7a:7a:00 eui.00253885910c8cec 0x500a07511bb48b25 alias.CDIMS1_A3:20:54:d0:39:ea:3d:b8:74
	// https://www.reddit.com/r/zfs/comments/1glttvl/validate_wwn/
	isValidWWNRegex = regexp.MustCompile(`^[0-9a-zA-Z._=:-]{5,64}$`)

	// ErrInvalidWWN indicates that a WWN has an invalid syntax.
	ErrInvalidWWN = fmt.Errorf("WWN does not match regex %q", isValidWWNRegex.String())
)

func (c *sshClient) WipeDisk(ctx context.Context, sliceOfWwns []string) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	if len(sliceOfWwns) == 0 {
		return "", nil
	}
	if slices.Contains(sliceOfWwns, "all") {
		out := c.runSSH("lsblk --nodeps --noheadings -o WWN | sort -u")
		if out.Err != nil {
			return "", fmt.Errorf("failed to find WWNs of all disks: %w", out.Err)
		}
		log.Info("WipeDisk: 'all' was given. Found these WWNs", "WWNs", sliceOfWwns)
		sliceOfWwns = strings.Fields(out.StdOut)
	} else {
		for _, wwn := range sliceOfWwns {
			// validate WWN.
			// It is unlikely, but someone could use this wwn: `"; do-nasty-things-here`
			if !isValidWWNRegex.MatchString(wwn) {
				return "", fmt.Errorf("WWN %q is invalid. %w", wwn, ErrInvalidWWN)
			}
		}
	}
	out := c.runSSH(fmt.Sprintf(`cat >/root/wipe-disk.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/wipe-disk.sh
/root/wipe-disk.sh %s
`, wipeDiskShellScript, strings.Join(sliceOfWwns, " ")))
	if out.Err != nil {
		return "", fmt.Errorf("WipeDisk for %+v failed: %s. %s: %w", sliceOfWwns, out.StdOut, out.StdErr, out.Err)
	}
	return out.String(), nil
}

func (c *sshClient) CheckDisk(_ context.Context, sliceOfWwns []string) (info string, err error) {
	if len(sliceOfWwns) == 0 {
		return "", nil
	}

	out := c.runSSH(fmt.Sprintf(`cat >/root/check-disk.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/check-disk.sh
/root/check-disk.sh %s
`, checkDiskShellScript, strings.Join(sliceOfWwns, " ")))
	exitStatus, err := out.ExitStatus()
	if err != nil {
		// Network error or similar. Script was not called.
		return "", fmt.Errorf("CheckDisk for %+v failed: %w", sliceOfWwns, err)
	}
	if exitStatus == 1 {
		// Script detected a broken disk.
		return "", fmt.Errorf("CheckDisk for %+v failed: %s. %s. %w %w", sliceOfWwns, out.StdOut, out.StdErr, out.Err, ErrCheckDiskBrokenDisk)
	}
	if exitStatus == 0 {
		// Everything was fine.
		return out.String(), nil
	}
	// Some other strange error like "unknown WWN"
	return "", fmt.Errorf("CheckDisk for %+v failed: %s. %s: %w", sliceOfWwns, out.StdOut, out.StdErr, out.Err)
}

func (c *sshClient) UntarTGZ() Output {
	// read tgz from container image.
	fileName := "/installimage.tgz"
	data, err := os.ReadFile(fileName)
	if err != nil {
		return Output{Err: fmt.Errorf("ReadInstallimageTgzFailed %s: %w", fileName, err)}
	}

	// send base64 encoded binary to machine via ssh.
	return c.runSSH(fmt.Sprintf("echo %s | base64 -d | tar -xzf-",
		base64.StdEncoding.EncodeToString(data)))
}

// IsConnectionRefusedError checks whether the ssh error is a connection refused error.
func IsConnectionRefusedError(err error) bool {
	return strings.Contains(err.Error(), ErrConnectionRefused.Error())
}

// IsAuthenticationFailedError checks whether the ssh error is an authentication failed error.
func IsAuthenticationFailedError(err error) bool {
	return strings.Contains(err.Error(), ErrAuthenticationFailed.Error())
}

// IsCommandExitedWithoutExitSignalError checks whether the ssh error is an unplanned exit error.
func IsCommandExitedWithoutExitSignalError(err error) bool {
	return strings.Contains(err.Error(), ErrCommandExitedWithoutExitSignal.Error())
}

// IsTimeoutError checks whether the ssh error is an unplanned exit error.
func IsTimeoutError(err error) bool {
	return strings.Contains(err.Error(), ErrTimeout.Error())
}

func (c *sshClient) runSSH(command string) Output {
	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey([]byte(c.privateSSHKey))
	if err != nil {
		return Output{Err: fmt.Errorf("unable to parse private key: %w", err)}
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //#nosec
		Timeout:         sshTimeOut,
	}

	// Connect to the remote server and perform the SSH handshake.

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%v", c.ip, c.port), config)
	if err != nil {
		return Output{Err: fmt.Errorf("failed to dial ssh. Error message: %s. DialErr: %w", err.Error(), errSSHDialFailed)}
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return Output{Err: fmt.Errorf("unable to create new ssh session: %w", err)}
	}
	defer sess.Close()

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer

	sess.Stdout = &stdoutBuffer
	sess.Stderr = &stderrBuffer

	err = sess.Run(command)
	return Output{
		StdOut: stdoutBuffer.String(),
		StdErr: stderrBuffer.String(),
		Err:    err,
	}
}

func removeUselessLinesFromCloudInitOutput(s string) string {
	regexes := []string{
		`^\s*\d+K [. ]+ \d+%.*\ds$`,                  //  10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s
		`^Get:\d+ https?://.* [kM]B.*`,               // Get:17 http://archive.ubuntu.com/ubuntu focal/universe Translation-en [5,124 kB[]`
		`^Preparing to unpack \.\.\..*`,              // Preparing to unpack .../04-libx11-6_2%!a(MISSING)1.6.9-2ubuntu1.6_amd64.deb ...\r
		`^Selecting previously unselected package.*`, // Selecting previously unselected package kubeadm.\r
		`^Setting up .* \.\.\..*`,                    // Setting up hicolor-icon-theme (0.17-2) ...\r
		`^Unpacking .* \.\.\..*`,                     // Unpacking libatk1.0-0:amd64 (2.35.1-1ubuntu2) ...\r
	}

	// Compile the regexes
	compiledRegexes := make([]*regexp.Regexp, 0, len(regexes))
	for _, re := range regexes {
		compiled, err := regexp.Compile(re)
		if err != nil {
			return fmt.Sprintf("removeUselessLinesFromCloudInitOutput: Failed to compile regex %s: %v\n%s",
				re, err, s)
		}
		compiledRegexes = append(compiledRegexes, compiled)
	}

	var output []string

	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line matches any of the regexes
		matches := false
		for _, re := range compiledRegexes {
			if re.MatchString(line) {
				matches = true
				break
			}
		}

		if matches {
			continue
		}
		output = append(output, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Sprintf("Error reading string: %v\n%s", err, s)
	}
	return strings.Join(output, "\n")
}

func (c *sshClient) ExecutePreProvisionCommand(ctx context.Context, command string) (int, string, error) {
	return 0, "todo ööö", nil
}
