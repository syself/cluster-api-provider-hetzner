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

type SSHInput struct {
	IP            string
	PrivateSSHKey string
	Port          int
}

type SSHOutput struct {
	StdOut string
	StdErr string
	Err    error
}
type SSHClient interface {
	GetHostName() SSHOutput
}

// SSHClientFactory is the interface for creating new SSHClient objects.
type SSHClientFactory interface {
	NewSSHClient(*Credentials) SSHClient
}

type sshClientFactory struct{}

var _ = SSHClientFactory(&sshClientFactory{})

func (f *sshClientFactory) NewSSHClient(creds Credentials) SSHClient {
	return &sshClient{
		ip:            hostData.IP,
		privateSSHKey: hostData.PrivateSSHKey,
		port:          hostData.Port,
	}
}

type sshClient struct {
	ip            string
	privateSSHKey string
	port          int
}

var _ = SSHClient(&sshClient{})

func (c *sshClient) GetHostName() SSHOutput {
	return runSSH("hostname", c.ip, c.port, c.privateSSHKey)
}

func runSSH(command, ip string, port int, privateSSHKey string) SSHOutput {

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey([]byte(privateSSHKey))
	if err != nil {
		return SSHOutput{Err: errors.Errorf("unable to parse private key: %v", err)}
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

	client, err := ssh.Dial("tcp", ip+":"+strconv.Itoa(port), config)
	if err != nil {
		return SSHOutput{Err: ErrSSHDialFailed}
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return SSHOutput{Err: errors.Wrap(err, "unable to create new ssh session")}
	}
	defer sess.Close()

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer

	sess.Stdout = &stdoutBuffer
	sess.Stderr = &stderrBuffer

	err = sess.Run(command)
	return SSHOutput{
		StdOut: stdoutBuffer.String(),
		StdErr: stderrBuffer.String(),
		Err:    err,
	}
}
