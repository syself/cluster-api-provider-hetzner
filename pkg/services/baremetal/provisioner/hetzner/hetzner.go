package hetznerclient

import (
	"github.com/go-logr/logr"
	robotclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/robot"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
	"github.com/syself/hrobot-go/models"
)

type hetznerConfig struct {
	// havePreprovImgBuilder            bool
	// deployKernelURL                  string
	// deployRamdiskURL                 string
	// deployISOURL                     string
	// liveISOForcePersistentBootDevice string
	// maxBusyHosts                     int
}

type hetznerProvisionerFactory struct {
	log                logr.Logger
	config             hetznerConfig
	robotClientFactory robotclient.Factory
	// Keep pointers to hetzner and inspector clients configured with
	// the global auth settings to reuse the connection between
	// reconcilers.
	// clientIronic    *gophercloud.ServiceClient
	// clientInspector *gophercloud.ServiceClient
}

func NewProvisionerFactory(robotClientFactory robotclient.Factory) provisioner.Factory {
	return hetznerProvisionerFactory{
		robotClientFactory: robotClientFactory,
	}
}

// NewProvisioner returns a new Hetzner Provisioner using the global
// configuration for finding the Hetzner services.
func (f hetznerProvisionerFactory) NewProvisioner(hostData provisioner.HostData) provisioner.Provisioner {
	return &hetznerProvisioner{
		robotClient: f.robotClientFactory.NewClient(hostData.RobotCredentials),
	}
}

// Provisioner implements the provisioning.Provisioner interface
// and uses Hetzner to manage the host.
type hetznerProvisioner struct {
	robotClient      robotclient.Client
	sshClientFactory sshclient.Factory
}

func (c *hetznerProvisioner) GetBMServer(id int) (*models.Server, error) {
	return c.robotClient.GetBMServer(id)
}

func (c *hetznerProvisioner) ListSSHKeys() ([]models.Key, error) {
	return c.robotClient.ListSSHKeys()
}
