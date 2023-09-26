/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// BareMetalHostFinalizer is the name of the finalizer added to
	// hosts to block delete operations until the physical host can be
	// deprovisioned.
	BareMetalHostFinalizer string = "hetznerbaremetalhost.infrastructure.cluster.x-k8s.io"

	// HostAnnotation is the key for an annotation that should go on a HetznerBareMetalMachine to
	// reference what HetznerBareMetalHost it corresponds to.
	HostAnnotation = "infrastructure.cluster.x-k8s.io/HetznerBareMetalHost"
)

// RootDeviceHints holds the hints for specifying the storage location
// for the root filesystem for the image. Need to specify either WWN or raid
// to provision host machine successfully.
type RootDeviceHints struct {
	// Unique storage identifier. The hint must match the actual value
	// exactly.
	// +optional
	WWN string `json:"wwn,omitempty"`
	// To specify multiple storage devices.
	// +optional
	Raid Raid `json:"raid,omitempty"`
}

// IsValid checks whether rootDeviceHint is valid.
func (rdh *RootDeviceHints) IsValid() bool {
	if rdh.WWN == "" && len(rdh.Raid.WWN) == 0 ||
		rdh.WWN != "" && len(rdh.Raid.WWN) > 0 {
		return false
	}
	return true
}

// ListOfWWN gives the list of WWNs - no matter if it's in WWN or Raid.
func (rdh *RootDeviceHints) ListOfWWN() []string {
	if rdh.WWN == "" {
		return rdh.Raid.WWN
	}
	return []string{rdh.WWN}
}

// Raid can be used instead of WWN to point to multiple storage devices.
type Raid struct {
	WWN []string `json:"wwn,omitempty"`
}

// ErrorType indicates the class of problem that has caused the Host resource
// to enter an error state.
type ErrorType string

const (
	// ErrorTypeSSHRebootTriggered is an error condition that the ssh reboot is triggered.
	ErrorTypeSSHRebootTriggered ErrorType = "ssh reboot triggered"
	// ErrorTypeSoftwareRebootTriggered is an error condition that the software reboot is triggered.
	ErrorTypeSoftwareRebootTriggered ErrorType = "software reboot triggered"
	// ErrorTypeHardwareRebootTriggered is an error condition that the hardware reboot is triggered.
	ErrorTypeHardwareRebootTriggered ErrorType = "hardware reboot triggered"

	// ErrorTypeConnectionError ErrorType is an error condition indicating that ssh command returned a connection refused error.
	ErrorTypeConnectionError ErrorType = "connection refused error of SSH command"

	// RegistrationError is an error condition occurring when the
	// controller is unable to retrieve information of a specific server via robot.
	RegistrationError ErrorType = "registration error"
	// PreparationError is an error condition occurring when something fails while preparing host reconciliation.
	PreparationError ErrorType = "preparation error"
	// ProvisioningError is an error condition occurring when the controller
	// fails to provision or deprovision the Host.
	ProvisioningError ErrorType = "provisioning error"

	// FatalError is a fatal error that triggers a failureMessage in the bm machine.
	FatalError ErrorType = "fatal error"

	// PermanentError is like fatal error but stays on host machine.
	PermanentError ErrorType = "permanent error"
)

const (
	// ErrorMessageMissingRootDeviceHints specifies the error message when no root device hints are specified.
	ErrorMessageMissingRootDeviceHints string = "no root device hints specified"
	// ErrorMessageMissingHetznerSecret specifies the error message when no Hetzner secret was found.
	ErrorMessageMissingHetznerSecret string = "could not find HetznerSecret"
	// ErrorMessageMissingRescueSSHSecret specifies the error message when no RescueSSH secret was found.
	ErrorMessageMissingRescueSSHSecret string = "could not find RescueSSHSecret"
	// ErrorMessageMissingOSSSHSecret specifies the error message when no OSSSH secret was found.
	ErrorMessageMissingOSSSHSecret string = "could not find OSSSHSecret"
	// ErrorMessageMissingOrInvalidSecretData specifies the error message when no data in secret is missing or invalid.
	ErrorMessageMissingOrInvalidSecretData string = "invalid or not specified information in secret"
)

// ProvisioningState defines the states the provisioner will report the host has having.
type ProvisioningState string

const (
	// StateNone means the state is unknown.
	StateNone ProvisioningState = ""

	// StatePreparing means we are checking if server exists and prepare it.
	StatePreparing ProvisioningState = "preparing"

	// StateRegistering means we are getting hardware details.
	StateRegistering ProvisioningState = "registering"

	// StateImageInstalling means we install a new image.
	StateImageInstalling ProvisioningState = "image-installing"

	// StateProvisioning means we are sending userData to the host and boot the machine.
	StateProvisioning ProvisioningState = "provisioning"

	// StateEnsureProvisioned means we are ensuring the reboot worked and cloud init is installed.
	StateEnsureProvisioned ProvisioningState = "ensure-provisioned"

	// StateProvisioned means we have sent userData to the host and booted the machine.
	StateProvisioned ProvisioningState = "provisioned"

	// StateDeprovisioning means we are removing all machine-specific information from host.
	StateDeprovisioning ProvisioningState = "deprovisioning"

	// StateDeleting means we are deleting the host.
	StateDeleting ProvisioningState = "deleting"
)

// RebootType defines the reboot type of servers via Hetzner robot API.
type RebootType string

const (
	// RebootTypeHardware defines the hardware reboot.
	RebootTypeHardware RebootType = "hw"
	// RebootTypePower defines the power reboot.
	RebootTypePower RebootType = "power"
	// RebootTypeSoftware defines the software reboot.
	RebootTypeSoftware RebootType = "sw"
	// RebootTypeManual defines the manual reboot.
	RebootTypeManual RebootType = "man"
)

// RebootAnnotationArguments defines the arguments of the RebootAnnotation type.
type RebootAnnotationArguments struct {
	Type RebootType `json:"type"`
}

// HetznerBareMetalHostSpec defines the desired state of HetznerBareMetalHost.
type HetznerBareMetalHostSpec struct {
	// ServerID defines the ID of the server provided by Hetzner.
	ServerID int `json:"serverID"`

	// Provide guidance about how to choose the device for the image
	// being provisioned. They need to be specified to provision the host.
	// +optional
	RootDeviceHints *RootDeviceHints `json:"rootDeviceHints,omitempty"`

	// ConsumerRef can be used to store information about something
	// that is using a host. When it is not empty, the host is considered "in use".
	// +optional
	ConsumerRef *corev1.ObjectReference `json:"consumerRef,omitempty"`

	// MaintenanceMode indicates that a machine is supposed to be deprovisioned
	// and won't be selected by any Hetzner bare metal machine.
	MaintenanceMode *bool `json:"maintenanceMode,omitempty"`

	// Description is a human-entered text used to help identify the host
	// +optional
	Description string `json:"description,omitempty"`

	// Status contains all status information. DO NOT EDIT!!!
	// +optional
	Status ControllerGeneratedStatus `json:"status,omitempty"`
}

// ControllerGeneratedStatus contains all status information which is important to persist.
type ControllerGeneratedStatus struct {
	// HetznerClusterRef is the name of the HetznerCluster object which is
	// needed as some necessary information is stored there, e.g. the hrobot password
	HetznerClusterRef string `json:"hetznerClusterRef"`

	// UserData holds the reference to the Secret containing the user
	// data to be passed to the host before it boots.
	// +optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// InstallImage is the configuration which is used for the autosetup configuration for installing an OS via InstallImage.
	// +optional
	InstallImage *InstallImage `json:"installImage,omitempty"`

	// StatusHardwareDetails are automatically gathered and should not be modified by the user.
	// +optional
	HardwareDetails *HardwareDetails `json:"hardwareDetails,omitempty"`

	// IPv4 address of server.
	// +optional
	IPv4 string `json:"ipv4"`

	// IPv6 address of server.
	// +optional
	IPv6 string `json:"ipv6"`

	// RebootTypes is a list of all available reboot types for API reboots
	// +optional
	RebootTypes []RebootType `json:"rebootTypes,omitempty"`

	// SSHSpec defines specs for SSH.
	SSHSpec *SSHSpec `json:"sshSpec,omitempty"`

	// HetznerRobotSSHKey contains name and fingerprint of the in HetznerCluster spec specified SSH key.
	// +optional
	SSHStatus SSHStatus `json:"sshStatus,omitempty"`

	// ErrorType indicates the type of failure encountered when the
	// OperationalStatus is OperationalStatusError
	// +optional
	ErrorType ErrorType `json:"errorType,omitempty"`

	// ErrorCount records how many times the host has encoutered an error since the last successful operation.
	// +kubebuilder:default:=0
	ErrorCount int `json:"errorCount"`

	// Information tracked by the provisioner.
	// +optional
	ProvisioningState ProvisioningState `json:"provisioningState,omitempty"`

	// the last error message reported by the provisioning subsystem.
	// +optional
	ErrorMessage string `json:"errorMessage"`

	// the last error message reported by the provisioning subsystem.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Rebooted shows whether the server is currently being rebooted.
	Rebooted bool `json:"rebooted,omitempty"`

	// Conditions defines current service state of the HetznerBareMetalHost.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// GetIPAddress returns the IPv6 if set, otherwise the IPv4.
func (sts ControllerGeneratedStatus) GetIPAddress() string {
	if sts.IPv4 == "" {
		return sts.IPv6
	}
	return sts.IPv4
}

// GetConditions returns the observations of the operational state of the HetznerBareMetalHost resource.
func (host *HetznerBareMetalHost) GetConditions() clusterv1.Conditions {
	return host.Spec.Status.Conditions
}

// SetConditions sets the underlying service state of the HetznerBareMetalHost to the predescribed clusterv1.Conditions.
func (host *HetznerBareMetalHost) SetConditions(conditions clusterv1.Conditions) {
	host.Spec.Status.Conditions = conditions
}

// SSHStatus contains all status information about SSHStatus.
type SSHStatus struct {
	// CurrentRescue gives information about the secret where the rescue ssh key is stored.
	CurrentRescue *SecretStatus `json:"currentRescue,omitempty"`
	// CurrentOS gives information about the secret where the os ssh key is stored.
	CurrentOS *SecretStatus `json:"currentOS,omitempty"`
	// OSKey contains name and fingerprint of the in HetznerBareMetalMachine spec specified SSH key.
	OSKey *SSHKey `json:"osKey,omitempty"`
	// RescueKey contains name and fingerprint of the in HetznerCluster spec specified SSH key.
	RescueKey *SSHKey `json:"rescueKey,omitempty"`
}

// SecretStatus contains the reference and version of the last secret that was used.
type SecretStatus struct {
	Reference *corev1.SecretReference `json:"credentials,omitempty"`
	Version   string                  `json:"credentialsVersion,omitempty"`
	DataHash  []byte                  `json:"credentialsDataHash,omitempty"`
}

// Match compares the saved status information with the name and
// content of a secret object. Returns false if an error occurred.
func (cs SecretStatus) Match(secret corev1.Secret) bool {
	switch {
	case cs.Reference == nil:
		return false
	case cs.Reference.Name != secret.ObjectMeta.Name:
		return false
	case cs.Reference.Namespace != secret.ObjectMeta.Namespace:
		return false
	}

	hash, err := HashOfSecretData(secret.Data)
	if err != nil {
		return false
	}

	return bytes.Equal(cs.DataHash, hash)
}

// Capacity is a disk size in Bytes.
type Capacity int64

// ClockSpeed is a clock speed in MHz
// +kubebuilder:validation:Format=double
type ClockSpeed string

// CPU describes one processor on the host.
type CPU struct {
	Arch           string     `json:"arch,omitempty"`
	Model          string     `json:"model,omitempty"`
	ClockGigahertz ClockSpeed `json:"clockGigahertz,omitempty"`
	Flags          []string   `json:"flags,omitempty"`
	Threads        int        `json:"threads,omitempty"`
	Cores          int        `json:"cores,omitempty"`
}

// Storage describes one storage device (disk, SSD, etc.) on the host.
type Storage struct {
	// The Linux device name of the disk, e.g. "/dev/sda". Note that this
	// may not be stable across reboots.
	Name string `json:"name,omitempty"`

	// The size of the disk in Bytes
	SizeBytes Capacity `json:"sizeBytes,omitempty"`

	// The size of the disk in GB
	SizeGB Capacity `json:"sizeGB,omitempty"`

	// The name of the vendor of the device
	Vendor string `json:"vendor,omitempty"`

	// Hardware model
	Model string `json:"model,omitempty"`

	// The serial number of the device
	SerialNumber string `json:"serialNumber,omitempty"`

	// The WWN of the device
	WWN string `json:"wwn,omitempty"`

	// The SCSI location of the device
	HCTL string `json:"hctl,omitempty"`

	// Rota defines if its a HDD device or not.
	Rota bool `json:"rota,omitempty"`
}

// NIC describes one network interface on the host.
type NIC struct {
	// The name of the network interface, e.g. "en0"
	Name string `json:"name,omitempty"`

	// The vendor and product IDs of the NIC, e.g. "0x8086 0x1572"
	Model string `json:"model,omitempty"`

	// The device MAC address
	// +kubebuilder:validation:Pattern=`[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}`
	MAC string `json:"mac,omitempty"`

	// The IP address of the interface. This will be an IPv4 or IPv6 address
	// if one is present.  If both IPv4 and IPv6 addresses are present in a
	// dual-stack environment, two nics will be output, one with each IP.
	IP string `json:"ip,omitempty"`

	// The speed of the device in Gigabits per second
	SpeedMbps int `json:"speedMbps,omitempty"`
}

// HardwareDetails collects all of the information about hardware
// discovered on the host.
type HardwareDetails struct {
	RAMGB   int       `json:"ramGB,omitempty"`
	NIC     []NIC     `json:"nics,omitempty"`
	Storage []Storage `json:"storage,omitempty"`
	CPU     CPU       `json:"cpu,omitempty"`
}

// HetznerBareMetalHostStatus defines the observed state of HetznerBareMetalHost.
type HetznerBareMetalHostStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hbmh;hbmhost
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".spec.status.provisioningState",description="Provisioning status"
// +kubebuilder:printcolumn:name="IPv4",type="string",JSONPath=".spec.status.ipv4",description="IPv4 of the host"
// +kubebuilder:printcolumn:name="IPv6",type="string",JSONPath=".spec.status.ipv6",description="IPv6 of the host"
// +kubebuilder:printcolumn:name="Threads",type="string",JSONPath=".spec.status.hardwareDetails.cpu.threads",description="CPU threads"
// +kubebuilder:printcolumn:name="Clock speed",type="string",JSONPath=".spec.status.hardwareDetails.cpu.clockGigahertz",description="CPU clock speed"
// +kubebuilder:printcolumn:name="RAM in GB",type="string",JSONPath=".spec.status.hardwareDetails.ramGB",description="RAM in GB"
// +kubebuilder:printcolumn:name="Consumer",type="string",JSONPath=".spec.consumerRef.name",description="Consumer using this host"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of BaremetalHost"
// +k8s:defaulter-gen=true

// HetznerBareMetalHost is the Schema for the hetznerbaremetalhosts API.
type HetznerBareMetalHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HetznerBareMetalHostSpec   `json:"spec,omitempty"`
	Status HetznerBareMetalHostStatus `json:"status,omitempty"`
}

// UpdateRescueSSHStatus modifies the ssh status with the last chosen ssh secret.
func (host *HetznerBareMetalHost) UpdateRescueSSHStatus(secret corev1.Secret) error {
	status, err := statusFromSecret(secret)
	if err != nil {
		return fmt.Errorf("failed get status from secret: %w", err)
	}
	host.Spec.Status.SSHStatus.CurrentRescue = status
	return nil
}

// UpdateOSSSHStatus modifies the ssh status with the last chosen ssh secret.
func (host *HetznerBareMetalHost) UpdateOSSSHStatus(secret corev1.Secret) error {
	status, err := statusFromSecret(secret)
	if err != nil {
		return fmt.Errorf("failed get status from secret: %w", err)
	}
	host.Spec.Status.SSHStatus.CurrentOS = status
	return nil
}

// statusFromSecret modifies the ssh status with information from ssh secret.
func statusFromSecret(secret corev1.Secret) (*SecretStatus, error) {
	hash, err := HashOfSecretData(secret.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash of data")
	}
	return &SecretStatus{
		Reference: &corev1.SecretReference{
			Name:      secret.ObjectMeta.Name,
			Namespace: secret.ObjectMeta.Namespace,
		},
		DataHash: hash,
	}, nil
}

// HashOfSecretData returns the sha256 of secret data.
func HashOfSecretData(data map[string][]byte) ([]byte, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	if _, err := hash.Write(b); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

// HasSoftwareReboot returns a boolean indicating whether software reboot exists for server.
func (host *HetznerBareMetalHost) HasSoftwareReboot() bool {
	for _, rt := range host.Spec.Status.RebootTypes {
		if rt == RebootTypeSoftware {
			return true
		}
	}
	return false
}

// HasHardwareReboot returns a boolean indicating whether hardware reboot exists for server.
func (host *HetznerBareMetalHost) HasHardwareReboot() bool {
	for _, rt := range host.Spec.Status.RebootTypes {
		if rt == RebootTypeHardware {
			return true
		}
	}
	return false
}

// HasPowerReboot returns a boolean indicating whether power reboot exists for server.
func (host *HetznerBareMetalHost) HasPowerReboot() bool {
	for _, rt := range host.Spec.Status.RebootTypes {
		if rt == RebootTypePower {
			return true
		}
	}
	return false
}

// NeedsProvisioning compares the settings with the provisioning
// status and returns true when more work is needed or false
// otherwise.
func (host *HetznerBareMetalHost) NeedsProvisioning() bool {
	// Without an image, there is nothing to provision.
	return host.Spec.Status.InstallImage != nil
}

// SetError updates the error type and message in the status struct and increases the ErrorCount.
func (host *HetznerBareMetalHost) SetError(errType ErrorType, errMessage string) {
	if errType == host.Spec.Status.ErrorType && errMessage == host.Spec.Status.ErrorMessage {
		host.Spec.Status.ErrorCount++
	} else {
		// new error - start fresh error count
		host.Spec.Status.ErrorCount = 1
	}
	host.Spec.Status.ErrorType = errType
	host.Spec.Status.ErrorMessage = errMessage
}

// ClearError removes the error on the host and resets the error count to 0.
func (host *HetznerBareMetalHost) ClearError() {
	var emptyErrType ErrorType
	if host.Spec.Status.ErrorType != emptyErrType {
		host.Spec.Status.ErrorType = emptyErrType
	}
	if host.Spec.Status.ErrorMessage != "" {
		host.Spec.Status.ErrorMessage = ""
	}
	host.Spec.Status.ErrorCount = 0
}

// HasRebootAnnotation checks for existence of reboot annotations and returns true if at least one exist.
func (host *HetznerBareMetalHost) HasRebootAnnotation() bool {
	for annotation := range host.GetAnnotations() {
		if isRebootAnnotation(annotation) {
			return true
		}
	}
	return false
}

// ClearRebootAnnotations deletes all reboot annotations that exist on the host.
func (host *HetznerBareMetalHost) ClearRebootAnnotations() {
	for annotation := range host.Annotations {
		if isRebootAnnotation(annotation) {
			delete(host.Annotations, annotation)
		}
	}
}

func isRebootAnnotation(annotation string) bool {
	return strings.HasPrefix(annotation, RebootAnnotation+"/") || annotation == RebootAnnotation
}

//+kubebuilder:object:root=true

// HetznerBareMetalHostList contains a list of HetznerBareMetalHost.
type HetznerBareMetalHostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerBareMetalHost `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HetznerBareMetalHost{}, &HetznerBareMetalHostList{})
}
