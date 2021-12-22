package provisioner

import (
	"time"

	"github.com/pkg/errors"
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
	NewSSHClient(hostData *HostData) SSHClient
}
