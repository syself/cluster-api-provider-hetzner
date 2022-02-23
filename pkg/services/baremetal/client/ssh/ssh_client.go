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
	return runSSH("hostname", c.ip, c.port, c.privateSSHKey)
}

func runSSH(command, ip string, port int, privateSSHKey string) Output {

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey([]byte(privateSSHKey))
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

	client, err := ssh.Dial("tcp", ip+":"+strconv.Itoa(port), config)
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
