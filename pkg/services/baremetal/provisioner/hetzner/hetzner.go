package hetzner

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
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
	log    logr.Logger
	config hetznerConfig

	// Keep pointers to hetzner and inspector clients configured with
	// the global auth settings to reuse the connection between
	// reconcilers.
	// clientIronic    *gophercloud.ServiceClient
	// clientInspector *gophercloud.ServiceClient
}

// Provisioner implements the provisioning.Provisioner interface
// and uses Hetzner to manage the host.
type hetznerProvisioner struct {
	// // the global hetzner settings
	// config hetznerConfig
	// // the object metadata of the BareMetalHost resource
	// objectMeta metav1.ObjectMeta
	// // the UUID of the node in Hetzner
	// nodeID string
	// // the address of the BMC
	// bmcAddress string
	// // whether to disable SSL certificate verification
	// disableCertVerification bool
	// // the MAC address of the PXE boot interface
	// bootMACAddress string
	// // a logger configured for this host
	// log logr.Logger
	// // a debug logger configured for this host
	// debugLog logr.Logger
}

func (f *hetznerProvisionerFactory) init(havePreprovImgBuilder bool) error {
	// ironicAuth, inspectorAuth, err := clients.LoadAuth()
	// if err != nil {
	// 	return err
	// }

	// f.config, err = loadConfigFromEnv(havePreprovImgBuilder)
	// if err != nil {
	// 	return err
	// }

	// ironicEndpoint, inspectorEndpoint, err := loadEndpointsFromEnv()
	// if err != nil {
	// 	return err
	// }

	// tlsConf := loadTLSConfigFromEnv()

	// f.log.Info("ironic settings",
	// 	"endpoint", ironicEndpoint,
	// 	"ironicAuthType", ironicAuth.Type,
	// 	"inspectorEndpoint", inspectorEndpoint,
	// 	"inspectorAuthType", inspectorAuth.Type,
	// 	"deployKernelURL", f.config.deployKernelURL,
	// 	"deployRamdiskURL", f.config.deployRamdiskURL,
	// 	"deployISOURL", f.config.deployISOURL,
	// 	"liveISOForcePersistentBootDevice", f.config.liveISOForcePersistentBootDevice,
	// 	"CACertFile", tlsConf.TrustedCAFile,
	// 	"ClientCertFile", tlsConf.ClientCertificateFile,
	// 	"ClientPrivKeyFile", tlsConf.ClientPrivateKeyFile,
	// 	"TLSInsecure", tlsConf.InsecureSkipVerify,
	// 	"SkipClientSANVerify", tlsConf.SkipClientSANVerify,
	// )

	// f.clientIronic, err = clients.IronicClient(
	// 	ironicEndpoint, ironicAuth, tlsConf)
	// if err != nil {
	// 	return err
	// }

	// f.clientInspector, err = clients.InspectorClient(
	// 	inspectorEndpoint, inspectorAuth, tlsConf)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func NewProvisionerFactory(havePreprovImgBuilder bool) provisioner.Factory {
	factory := hetznerProvisionerFactory{}

	err := factory.init(havePreprovImgBuilder)
	if err != nil {
		factory.log.Error(err, "Cannot start hetzner provisioner")
		os.Exit(1)
	}
	return factory
}

func (f hetznerProvisionerFactory) hetznerProvisioner(hostData provisioner.HostData) (*hetznerProvisioner, error) {

	p := &hetznerProvisioner{}

	return p, nil
}

// NewProvisioner returns a new Hetzner Provisioner using the global
// configuration for finding the Hetzner services.
func (f hetznerProvisionerFactory) NewProvisioner(hostData provisioner.HostData) (provisioner.Provisioner, error) {
	return f.hetznerProvisioner(hostData)
}
