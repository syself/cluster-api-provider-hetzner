package provisioner

import (
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client"
	"github.com/syself/hrobot-go/models"
)

// EventPublisher is a function type for publishing events associated
// with provisioning.
type EventPublisher func(reason, message string)

// Factory is the interface for creating new Provisioner objects.
type Factory interface {
	NewProvisioner(hostData HostData) Provisioner
}

// Provisioner holds the state information for talking to the
// provisioning backend.
type Provisioner interface {
	GetBMServer(int) (*models.Server, error)

	ListSSHKeys() ([]models.Key, error)
	// Provision writes the image from the host spec to the host. It
	// may be called multiple times, and should return true for its
	// dirty flag until the provisioning operation is completed.
	//Provision(data ProvisionData) (result Result, err error)
}

type HostData struct {
	RobotCredentials robotclient.RobotCredentials
	PrivateSSHKey    string // TODO: Update to actual ssh object containing all relevant information
	ServerID         int
	IP               string // TODO: What information does the SSH client need that we can provide on client setup?
	Port             int
}

func NewHostData(host *infrav1.HetznerBareMetalHost, robotCreds robotclient.RobotCredentials) *HostData {
	return &HostData{
		RobotCredentials: robotCreds,
		ServerID:         host.Spec.ServerID,
	}
}

// Result holds the response from a call in the Provsioner API.
type Result struct {
	// RequeueAfter indicates how long to wait before making the same
	// Provisioner call again. The request should only be requeued if
	// Dirty is also true.
	RequeueAfter time.Duration
	// Any error message produced by the provisioner.
	ErrorMessage string
}

func BuildHostData(creds robotclient.RobotCredentials, sshCreds robotclient.SSHCredentials) HostData {
	return HostData{
		RobotCredentials: creds,
		PrivateSSHKey:    sshCreds.PrivateKey,
	}
}
