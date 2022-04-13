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
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const (
	sshTimeOut time.Duration = 5 * time.Second
)

var (
	// ErrCommandExitedWithoutExitSignal means the ssh command exited unplanned.
	ErrCommandExitedWithoutExitSignal = errors.New("wait: remote command exited without exit status or exit signal")
	// ErrCommandExitedWithStatusOne means the ssh command exited with sttatus 1.
	ErrCommandExitedWithStatusOne = errors.New("Process exited with status 1")

	// ErrConnectionRefused means the ssh connection was refused.
	ErrConnectionRefused = errors.New("connect: connection refused")
	// ErrAuthenticationFailed means ssh was unable to authenticate.
	ErrAuthenticationFailed = errors.New("ssh: unable to authenticate")
	// ErrEmptyStdOut means that StdOut equals empty string.
	ErrEmptyStdOut = errors.New("unexpected empty output in stdout")
	// ErrTimeout means that there is a timeout error.
	ErrTimeout = errors.New("i/o timeout")

	errSSHDialFailed = errors.New("failed to dial ssh")
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
	CreateAutoSetup(data string) Output
	DownloadImage(path, url string) Output
	CreatePostInstallScript(data string) Output
	ExecuteInstallImage(hasPostInstallScript bool) Output
	Reboot() Output
	EnsureCloudInit() Output
	CreateNoCloudDirectory() Output
	CreateMetaData(hostName string) Output
	CreateUserData(userData string) Output
	CloudInitStatus() Output
	CheckCloudInitLogsForSigTerm() Output
	CleanCloudInitLogs() Output
	CleanCloudInitInstances() Output
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
	out := c.runSSH(`cat << 'EOF' > nic-info.sh
#!/bin/sh
for iname in $(ip a |awk '/state UP/{print $2}' | sed 's/://')
do

MAC=\""$(ip a | grep -A2 $iname | awk '/link/{print $2}')\""
SPEED=\""$(ethtool eth0 |grep "Speed:" | awk '{print $2}' | sed 's/[^0-9]//g')\""
MODEL=\""$( lspci | grep net | awk '{$1=$2=$3=""; print $0}' | sed "s/^[ \t]*//")\""
IP_V4=\""$(ip a | grep -A2 eth0 | sed -n '/\binet\b/p' | awk '{print $2}')\""
IP_V6=\""$(ip a | grep -A2 eth0 | sed -n '/\binet6\b/p' | awk '{print $2}')\""

if test -n $IP_V4; then
	echo "name=\""$iname\""" "model=$MODEL" "mac=$MAC" "ip=$IP_V4" "speedMbps=$SPEED"
fi

if test -n $IP_V6; then
	echo "name=\""$iname\""" "model=$MODEL" "mac=$MAC" "ip=$IP_V6" "speedMbps=$SPEED"  
fi

done
EOF`)
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	return c.runSSH("sh nic-info.sh")
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

// GetHardwareDetailsCPUThreads implements the GetHardwareDetailsCPUThreads method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUThreads() Output {
	return c.runSSH(`lscpu | grep "CPU(s):" | head -1 |  awk '{ print $2}'`)
}

// GetHardwareDetailsCPUCores implements the GetHardwareDetailsCPUCores method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUCores() Output {
	return c.runSSH(`grep 'cpu cores' /proc/cpuinfo | uniq | awk '{print $4}'`)
}

// CreateAutoSetup implements the CreateAutoSetup method of the SSHClient interface.
func (c *sshClient) CreateAutoSetup(data string) Output {
	return c.runSSH(fmt.Sprintf(`cat << 'EOF' > /autosetup 
%s
EOF`, data))
}

// DownloadImage implements the DownloadImage method of the SSHClient interface.
func (c *sshClient) DownloadImage(path, url string) Output {
	return c.runSSH(fmt.Sprintf(`curl -sLo "%s" "%s"`, path, url))
}

// CreatePostInstallScript implements the CreatePostInstallScript method of the SSHClient interface.
func (c *sshClient) CreatePostInstallScript(data string) Output {
	out := c.runSSH(fmt.Sprintf(`cat << 'EOF' > /root/post-install.sh 
	%sEOF`, data))

	if out.Err != nil || out.StdErr != "" {
		return out
	}
	return c.runSSH(`chmod +x /root/post-install.sh . `)
}

// ExecuteInstallImage implements the ExecuteInstallImage method of the SSHClient interface.
func (c *sshClient) ExecuteInstallImage(hasPostInstallScript bool) Output {
	var cmd string
	if hasPostInstallScript {
		cmd = `/root/.oldroot/nfs/install/installimage -a -c /autosetup -x /root/post-install.sh`
	} else {
		cmd = `/root/.oldroot/nfs/install/installimage -a -c /autosetup`
	}

	out := c.runSSH(fmt.Sprintf(`cat << 'EOF' > /root/install-image-script.sh 
#!/bin/bash
export TERM=xterm
%s
EOF`, cmd))
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	out = c.runSSH(`chmod +x /root/install-image-script.sh . `)
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	out = c.runSSH(`sh /root/install-image-script.sh`)
	if out.Err != nil {
		return out
	}
	// Ignore StdErr in this command
	return Output{StdOut: out.StdOut}
}

// Reboot implements the Reboot method of the SSHClient interface.
func (c *sshClient) Reboot() Output {
	out := c.runSSH(`reboot`)
	if out.Err != nil && strings.Contains(out.Err.Error(), ErrCommandExitedWithoutExitSignal.Error()) {
		return Output{}
	}
	return out
}

// EnsureCloudInit implements the EnsureCloudInit method of the SSHClient interface.
func (c *sshClient) EnsureCloudInit() Output {
	return c.runSSH(`command -v cloud-init`)
}

// CreateNoCloudDirectory implements the CreateNoCloudDirectory method of the SSHClient interface.
func (c *sshClient) CreateNoCloudDirectory() Output {
	return c.runSSH(`mkdir -p /var/lib/cloud/seed/nocloud-net`)
}

// CreateMetaData implements the CreateMetaData method of the SSHClient interface.
func (c *sshClient) CreateMetaData(hostName string) Output {
	return c.runSSH(fmt.Sprintf(`cat << 'EOF' > /var/lib/cloud/seed/nocloud-net/meta-data
local-hostname: %s
EOF`, hostName))
}

// CreateUserData implements the CreateUserData method of the SSHClient interface.
func (c *sshClient) CreateUserData(userData string) Output {
	return c.runSSH(fmt.Sprintf(`cat << 'EOF' > /var/lib/cloud/seed/nocloud-net/user-data
%sEOF`, userData))
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
		return Output{Err: errors.Errorf("unable to parse private key: %v", err)}
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

	client, err := ssh.Dial("tcp", c.ip+":"+strconv.Itoa(c.port), config)
	if err != nil {
		return Output{Err: fmt.Errorf("failed to dial ssh. Error message: %s. DialErr: %w", err.Error(), errSSHDialFailed)}
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return Output{Err: errors.Wrap(err, "unable to create new ssh session")}
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
