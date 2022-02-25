package sshclient

import (
	"bytes"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const (
	sshTimeOut time.Duration = 5 * time.Second
)

var (
	ErrSSHDialFailed = errors.New("failed to dial ssh")
)

type Input struct {
	IP         string
	PrivateKey string
	Port       int
}

type Output struct {
	StdOut string
	StdErr string
	Err    error
}
type Client interface {
	GetHostName() Output
	GetHardwareDetailsRam() Output
	GetHardwareDetailsNics() Output
	GetHardwareDetailsStorage() Output
	GetHardwareDetailsCPUArch() Output
	GetHardwareDetailsCPUModel() Output
	GetHardwareDetailsCPUClockGigahertz() Output
	GetHardwareDetailsCPUFlags() Output
	GetHardwareDetailsCPUThreads() Output
	GetHardwareDetailsCPUCores() Output
}

// Factory is the interface for creating new Client objects.
type Factory interface {
	NewClient(Credentials) Client
}

type sshFactory struct{}

var _ = Factory(&sshFactory{})

func (f *sshFactory) NewClient(creds Credentials) Client {
	return &sshClient{
		privateSSHKey: creds.PrivateKey,
	}
}

type sshClient struct {
	ip            string
	privateSSHKey string
	port          int
}

var _ = Client(&sshClient{})

func (c *sshClient) GetHostName() Output {
	return c.runSSH("hostname")
}

func (c *sshClient) GetHardwareDetailsRam() Output {
	return c.runSSH("grep MemTotal /proc/meminfo | awk '{print $2}'")
}

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

func (c *sshClient) GetHardwareDetailsStorage() Output {
	return c.runSSH(`lsblk -b -P -o "NAME,LABEL,FSTYPE,TYPE,HCTL,MODEL,VENDOR,SERIAL,SIZE,WWN,ROTA"`)
}

func (c *sshClient) GetHardwareDetailsCPUArch() Output {
	return c.runSSH(`lscpu | grep "Architecture:" | awk '{print $2}'`)
}

func (c *sshClient) GetHardwareDetailsCPUModel() Output {
	return c.runSSH(`lscpu | grep "Model name:" | awk '{$1=$2=""; print $0}' | sed "s/^[ \t]*//"`)
}

func (c *sshClient) GetHardwareDetailsCPUClockGigahertz() Output {
	return c.runSSH(`lscpu | grep "CPU max MHz:" |  awk '{printf "%.1f", $4/1000}'`)
}

func (c *sshClient) GetHardwareDetailsCPUFlags() Output {
	return c.runSSH(`lscpu | grep "Flags:" |  awk '{ $1=""; print $0}' | sed "s/^[ \t]*//"`)
}

func (c *sshClient) GetHardwareDetailsCPUThreads() Output {
	return c.runSSH(`lscpu | grep "CPU(s):" | head -1 |  awk '{ print $2}'`)
}

func (c *sshClient) GetHardwareDetailsCPUCores() Output {
	return c.runSSH(`grep 'cpu cores' /proc/cpuinfo | uniq | awk '{print $4}'`)
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ssh.FixedHostKey(hostKey),
		Timeout:         sshTimeOut,
	}

	// Connect to the remote server and perform the SSH handshake.

	client, err := ssh.Dial("tcp", c.ip+":"+strconv.Itoa(c.port), config)
	if err != nil {
		return Output{Err: ErrSSHDialFailed}
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
