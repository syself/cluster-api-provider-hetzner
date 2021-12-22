package provisioner

import (
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

type HostData struct {
	// ObjectMeta                     metav1.ObjectMeta
	// BMCAddress                     string
	// BMCCredentials                 bmc.Credentials
	// DisableCertificateVerification bool
	// BootMACAddress                 string
	// ProvisionerID                  string
}

// EventPublisher is a function type for publishing events associated
// with provisioning.
type EventPublisher func(reason, message string)

// Factory is the interface for creating new Provisioner objects.
type Factory interface {
	NewProvisioner(hostData HostData) (Provisioner, error)
}

// Provisioner holds the state information for talking to the
// provisioning backend.
type Provisioner interface {

	// Provision writes the image from the host spec to the host. It
	// may be called multiple times, and should return true for its
	// dirty flag until the provisioning operation is completed.
	//Provision(data ProvisionData) (result Result, err error)
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

func BuildHostData(host infrav1.HetznerBareMetalHost) HostData {
	return HostData{}
}
