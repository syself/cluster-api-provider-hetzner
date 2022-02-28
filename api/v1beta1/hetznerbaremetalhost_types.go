/*
Copyright 2021.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HetznerBareMetalHostFinalizer is the name of the finalizer added to
	// hosts to block delete operations until the physical host can be
	// deprovisioned.
	HetznerBareMetalHostFinalizer string = "hetznerbaremetalhost.infrastructure.cluster.x-k8s.io"

	// PausedAnnotation is the annotation that pauses the reconciliation (triggers
	// an immediate requeue).
	PausedAnnotation = "hetznerbaremetalhost.infrastructure.cluster.x-k8s.io/paused"

	// DetachedAnnotation is the annotation which stops provisioner management of the host
	// unlike in the paused case, the host status may be updated.
	DetachedAnnotation = "hetznerbaremetalhost.infrastructure.cluster.x-k8s.io/detached"
)

// RootDeviceHints holds the hints for specifying the storage location
// for the root filesystem for the image.
type RootDeviceHints struct {
	// Unique storage identifier. The hint must match the actual value
	// exactly.
	WWN string `json:"wwn,omitempty"`
}

// OperationalStatus represents the state of the host.
type OperationalStatus string

const (
	// OperationalStatusOK is the status value for when the host is
	// configured correctly and is manageable.
	OperationalStatusOK OperationalStatus = "OK"

	// OperationalStatusDiscovered is the status value for when the
	// host is only partially configured
	OperationalStatusDiscovered OperationalStatus = "discovered"

	// OperationalStatusError is the status value for when the host
	// has any sort of error.
	OperationalStatusError OperationalStatus = "error"

	// OperationalStatusDetached is the status value when the host is
	// marked unmanaged via the detached annotation.
	OperationalStatusDetached OperationalStatus = "detached"
)

// ErrorType indicates the class of problem that has caused the Host resource
// to enter an error state.
type ErrorType string

const (
	ErrorTypeSSHResetTooSlow      ErrorType = "ssh reset too slow"
	ErrorTypeSoftwareResetTooSlow ErrorType = "software reset too slow"
	ErrorTypeHardwareResetTooSlow ErrorType = "hardware reset too slow"

	ErrorTypeSSHResetNotStarted      ErrorType = "ssh reset not started"
	ErrorTypeSoftwareResetNotStarted ErrorType = "software reset not started"
	ErrorTypeHardwareResetNotStarted ErrorType = "hardware reset not started"

	ErrorTypeHardwareResetFailed ErrorType = "hardware reset failed"
	// ProvisionedRegistrationError is an error condition occurring when the controller
	// is unable to re-register an already provisioned host.
	ProvisionedRegistrationError ErrorType = "provisioned registration error"
	// RegistrationError is an error condition occurring when the
	// controller is unable to retrieve information of a specific server via robot.
	RegistrationError ErrorType = "registration error"
	// PreparationError is an error condition occurring when a machine fails running installimage.
	PreparationError ErrorType = "preparation error"
	// ProvisioningError is an error condition occurring when the controller
	// fails to provision or deprovision the Host.
	ProvisioningError ErrorType = "provisioning error"
	// DetachError is an error condition occurring when the
	// controller is unable to detatch the host from the provisioner.
	DetachError ErrorType = "detach error"
)

// ProvisioningState defines the states the provisioner will report the host has having.
type ProvisioningState string

const (
	// StateNone means the state is unknown.
	StateNone ProvisioningState = ""

	// StateUnmanaged means there is insufficient information available to register the host.
	StateUnmanaged ProvisioningState = "unmanaged"

	// StateRegistering means we are telling the backend about the host. Checking if server exists in robot api. -> available.
	StateRegistering ProvisioningState = "registering"

	// StateAvailable
	StateAvailable ProvisioningState = "available"

	// StateImageInstalling means we install a new image.
	StateImageInstalling ProvisioningState = "image-installing"

	// StateProvisioning means we are sending user_data to the host and boot the machine.
	StateProvisioning ProvisioningState = "provisioning"

	// StateProvisioned means we have sent user_data to the host and booted the machine.
	StateProvisioned ProvisioningState = "provisioned"

	// StateDeprovisioning means we are removing an image from the host's disk(s).
	StateDeprovisioning ProvisioningState = "deprovisioning"

	// StateDeleting means we are in the process of cleaning up the host ready for deletion.
	StateDeleting ProvisioningState = "deleting"
)

type ResetType string

const (
	ResetTypeHardware ResetType = "hw"
	ResetTypePower    ResetType = "power"
	ResetTypeSoftware ResetType = "sw"
	ResetTypeManual   ResetType = "man"
)

// HetznerBareMetalHostSpec defines the desired state of HetznerBareMetalHost.
type HetznerBareMetalHostSpec struct {
	// ServerID defines the ID of the server provided by Hetzner.
	ServerID int `json:"serverID,omitempty"`

	// Type of the server.
	Type string `json:"type,omitempty"`

	// Provide guidance about how to choose the device for the image
	// being provisioned.
	RootDeviceHints *RootDeviceHints `json:"rootDeviceHints,omitempty"`

	// ConsumerRef can be used to store information about something
	// that is using a host. When it is not empty, the host is
	// considered "in use".
	ConsumerRef *corev1.ObjectReference `json:"consumerRef,omitempty"`

	// Should the server be online?
	Online bool `json:"online"`

	AutoSetupTemplateRef *corev1.ConfigMapKeySelector `json:"autoSetupTemplateRef"`

	// Image holds the details of the image to be provisioned.
	// http, https, ftp, nfs allowed.
	// TODO: Validation ftp:// ; http:// ; https:// ;  hostname:
	Image string `json:"image,omitempty"`

	// MaintenanceMode indicates that a machine is supposed to be deprovisioned
	// and won't be selected by any Hetzner bare metal machine.
	MaintenanceMode bool `json:"maintenanceMode,omitempty"`

	// Description is a human-entered text used to help identify the host
	// +optional
	Description string `json:"description,omitempty"`

	// HetznerClusterName points to the name of the HetznerCluster object and is used internally.
	HetznerClusterName string `json:"hetznerClusterName"`

	// When set to true we delete all data when we provision the node.
	// +optional
	// +kubebuilder:default:=false
	CleanUpData bool `json:"cleanUpData,omitempty"`

	// HetznerClusterRef is the name of the HetznerCluster object which is
	// needed as some necessary information is stored there, e.g. the hrobot password
	HetznerClusterRef string `json:"hetznerClusterRef,omitempty"`

	// Status contains all status information. DO NOT EDIT!!!
	// +optional
	Status ControllerGeneratedStatus `json:"status,omitempty"`
}

// ControllerGeneratedStatus contains all status information which is important to persist
type ControllerGeneratedStatus struct {
	// UserData holds the reference to the Secret containing the user
	// data to be passed to the host before it boots.
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// StatusHardwareDetails are automatically gathered and should not be modified by the user.
	HardwareDetails *HardwareDetails `json:"hardwareDetails,omitempty"`

	// IP address of server.
	IP string `json:"ip"`

	// OperationalStatus holds the status of the host
	// +kubebuilder:validation:Enum="";OK;discovered;error;delayed;detached
	OperationalStatus OperationalStatus `json:"operationalStatus"`

	// ResetTypes is a list of all available reset types for API resets
	ResetTypes []ResetType `json:"resetTypes"`

	// HetznerRobotSSHKey contains name and fingerprint of the in HetznerCluster spec specified SSH key.
	HetznerRobotSSHKey *SSHKey `json:"hetznerRobotSSHKey,omitempty"`

	// the last credentials we were able to validate as working
	GoodSSHCredentials CredentialsStatus `json:"goodSSHCredentials,omitempty"`

	// the last credentials we sent to the provisioning backend
	TriedSSHCredentials CredentialsStatus `json:"triedSSHCredentials,omitempty"`

	// ErrorType indicates the type of failure encountered when the
	// OperationalStatus is OperationalStatusError
	// +kubebuilder:validation:Enum=provisioned registration error;registration error;preparation error;provisioning error
	ErrorType ErrorType `json:"errorType,omitempty"`

	// OperationHistory holds information about operations performed
	// on this host.
	OperationHistory OperationHistory `json:"operationHistory,omitempty"`

	// ErrorCount records how many times the host has encoutered an error since the last successful operation.
	// +kubebuilder:default:=0
	ErrorCount int `json:"errorCount"`

	// Information tracked by the provisioner.
	ProvisioningState ProvisioningState `json:"provisioningState"`

	// the last error message reported by the provisioning subsystem.
	ErrorMessage string `json:"errorMessage"`

	// the last error message reported by the provisioning subsystem.
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// HetznerBareMetalHostStatus defines the observed state of HetznerBareMetalHost.
type HetznerBareMetalHostStatus struct {
	// Region contains the server location.
	Region Region `json:"region,omitempty"`

	// indicator for whether or not the host is powered on.
	PoweredOn bool `json:"poweredOn"`
}

// CredentialsStatus contains the reference and version of the last
// set of credentials the controller was able to validate.
type CredentialsStatus struct {
	Reference *corev1.SecretReference `json:"credentials,omitempty"`
	Version   string                  `json:"credentialsVersion,omitempty"`
}

// Match compares the saved status information with the name and
// content of a secret object.
func (cs CredentialsStatus) Match(secret corev1.Secret) bool {
	switch {
	case cs.Reference == nil:
		return false
	case cs.Reference.Name != secret.ObjectMeta.Name:
		return false
	case cs.Reference.Namespace != secret.ObjectMeta.Namespace:
		return false
	case cs.Version != secret.ObjectMeta.ResourceVersion:
		return false
	}
	return true
}

// OperationHistory holds information about operations performed on a
// host.
type OperationHistory struct {
	Register    OperationMetric `json:"register,omitempty"`
	Prepare     OperationMetric `json:"prepare,omitempty"`
	Provision   OperationMetric `json:"provision,omitempty"`
	Deprovision OperationMetric `json:"deprovision,omitempty"`
}

// OperationMetric contains metadata about an operation (inspection,
// provisioning, etc.) used for tracking metrics.
type OperationMetric struct {
	// +nullable
	Start metav1.Time `json:"start,omitempty"`
	// +nullable
	End metav1.Time `json:"end,omitempty"`
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
// TODO: Could be matched by extracting the information with: lsblk -b -P -o "NAME,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,HCTL,ROTA" .
type Storage struct {
	// The Linux device name of the disk, e.g. "/dev/sda". Note that this
	// may not be stable across reboots.
	Name string `json:"name,omitempty"`

	// The size of the disk in Bytes
	SizeBytes Capacity `json:"sizeBytes,omitempty"`

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
	RAMMebibytes int       `json:"ramMebibytes,omitempty"`
	NIC          []NIC     `json:"nics,omitempty"`
	Storage      []Storage `json:"storage,omitempty"`
	CPU          CPU       `json:"cpu,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hbmh;hbmhost
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".spec.status.operationalStatus",description="Operational status",priority=1
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".spec.status.provisioning.state",description="Provisioning status"
// +kubebuilder:printcolumn:name="Consumer",type="string",JSONPath=".spec.status.consumerRef.name",description="Consumer using this host"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="The type of server",priority=1
// +kubebuilder:printcolumn:name="Online",type="string",JSONPath=".spec.online",description="Whether the host is online or not"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".spec.status.errorType",description="Type of the most recent error"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of BaremetalHost"

// HetznerBareMetalHost is the Schema for the hetznerbaremetalhosts API.
type HetznerBareMetalHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HetznerBareMetalHostSpec   `json:"spec,omitempty"`
	Status HetznerBareMetalHostStatus `json:"status,omitempty"`
}

// UpdateGoodCredentials modifies the GoodCredentials portion of the
// Status struct to record the details of the secret containing
// credentials known to work.
func (host *HetznerBareMetalHost) UpdateGoodSSHCredentials(currentSecret corev1.Secret) {
	host.Spec.Status.GoodSSHCredentials.Version = currentSecret.ObjectMeta.ResourceVersion
	host.Spec.Status.GoodSSHCredentials.Reference = &corev1.SecretReference{
		Name:      currentSecret.ObjectMeta.Name,
		Namespace: currentSecret.ObjectMeta.Namespace,
	}
}

// UpdateTriedCredentials modifies the TriedCredentials portion of the
// Status struct to record the details of the secret containing
// credentials known to work.
func (host *HetznerBareMetalHost) UpdateTriedSSHCredentials(currentSecret corev1.Secret) {
	host.Spec.Status.TriedSSHCredentials.Version = currentSecret.ObjectMeta.ResourceVersion
	host.Spec.Status.TriedSSHCredentials.Reference = &corev1.SecretReference{
		Name:      currentSecret.ObjectMeta.Name,
		Namespace: currentSecret.ObjectMeta.Namespace,
	}
}

func (host *HetznerBareMetalHost) HasSoftwareReset() bool {
	for _, rt := range host.Spec.Status.ResetTypes {
		if rt == ResetTypeSoftware {
			return true
		}
	}
	return false
}

func (host *HetznerBareMetalHost) HasHardwareReset() bool {
	for _, rt := range host.Spec.Status.ResetTypes {
		if rt == ResetTypeHardware {
			return true
		}
	}
	return false
}

func (host *HetznerBareMetalHost) HasPowerReset() bool {
	for _, rt := range host.Spec.Status.ResetTypes {
		if rt == ResetTypeHardware {
			return true
		}
	}
	return false
}

// NeedsProvisioning compares the settings with the provisioning
// status and returns true when more work is needed or false
// otherwise.
func (host *HetznerBareMetalHost) NeedsProvisioning() bool {
	if host.Spec.Image == "" {
		// Without an image, there is nothing to provision.
		return false
	}
	return false
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
