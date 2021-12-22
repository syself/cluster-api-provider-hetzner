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
// TODO: Could be gathered by lsblk -b -P -o "NAME,LABEL,FSTYPE,TYPE,HCTL,MODEL,VENDOR,SERIAL,SIZE,WWN,ROTA" .
type RootDeviceHints struct {
	// A Linux device name like "/dev/vda". The hint must match the
	// actual value exactly.
	DeviceName string `json:"deviceName,omitempty"`

	// A SCSI bus address like 0:0:0:0. The hint must match the actual
	// value exactly.
	HCTL string `json:"hctl,omitempty"`

	// A vendor-specific device identifier. The hint can be a
	// substring of the actual value.
	Model string `json:"model,omitempty"`

	// The name of the vendor or manufacturer of the device. The hint
	// can be a substring of the actual value.
	Vendor string `json:"vendor,omitempty"`

	// Device serial number. The hint must match the actual value
	// exactly.
	SerialNumber string `json:"serialNumber,omitempty"`

	// The minimum size of the device in Gigabytes.
	// +kubebuilder:validation:Minimum=0
	MinSizeGigabytes int `json:"minSizeGigabytes,omitempty"`

	// Unique storage identifier. The hint must match the actual value
	// exactly.
	WWN string `json:"wwn,omitempty"`

	// True if the device should use spinning media, false otherwise.
	Rotational *bool `json:"rotational,omitempty"`
}

// OperationalStatus represents the state of the host.
type OperationalStatus string

const (
	// OperationalStatusOK is the status value for when the host is
	// configured correctly and is manageable.
	OperationalStatusOK OperationalStatus = "OK"

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

	// StateAvailable means the host can be consumed.
	StateAvailable ProvisioningState = "available"

	// StatePreparing means we are removing existing configuration and install a new image.
	StatePreparing ProvisioningState = "preparing"

	// StatePrepared means we have installed a new image.
	StatePrepared ProvisioningState = "prepared"

	// StateProvisioning means we are sending user_data to the host and boot the machine.
	StateProvisioning ProvisioningState = "provisioning"

	// StateProvisioned means we have sent user_data to the host and booted the machine.
	StateProvisioned ProvisioningState = "provisioned"

	// StateDeprovisioning means we are removing an image from the host's disk(s).
	StateDeprovisioning ProvisioningState = "deprovisioning"

	// StateDeleting means we are in the process of cleaning up the host ready for deletion.
	StateDeleting ProvisioningState = "deleting"
)

// HetznerBareMetalHostSpec defines the desired state of HetznerBareMetalHost.
type HetznerBareMetalHostSpec struct {
	// ServerID defines the ID of the server provided by Hetzner.
	ServerID string `json:"serverID,omitempty"`

	// Type of the server.
	Type string `json:"type,omitempty"`

	// Region contains the server location.
	Region Region `json:"type,omitempty"`

	// Provide guidance about how to choose the device for the image
	// being provisioned.
	RootDeviceHints *RootDeviceHints `json:"rootDeviceHints,omitempty"`

	// Should the server be online?
	Online bool `json:"online"`

	// ConsumerRef can be used to store information about something
	// that is using a host. When it is not empty, the host is
	// considered "in use".
	ConsumerRef *corev1.ObjectReference `json:"consumerRef,omitempty"`

	// Image holds the details of the image to be provisioned.
	// http, https, ftp allowed or the name.
	Image string `json:"image,omitempty"`

	// UserData holds the reference to the Secret containing the user
	// data to be passed to the host before it boots.
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// Description is a human-entered text used to help identify the host
	// +optional
	Description string `json:"description,omitempty"`

	// When set to true we delete all data when we provision the node.
	// +optional
	// +kubebuilder:default:=false
	// +kubebuilder:validation:Optional
	CleanUpData bool `json:"cleanUpData,omitempty"`
}

// HetznerBareMetalHostStatus defines the observed state of HetznerBareMetalHost.
type HetznerBareMetalHostStatus struct {
	// OperationalStatus holds the status of the host
	// +kubebuilder:validation:Enum="";OK;discovered;error;delayed;detached
	OperationalStatus OperationalStatus `json:"operationalStatus"`

	// ErrorType indicates the type of failure encountered when the
	// OperationalStatus is OperationalStatusError
	// +kubebuilder:validation:Enum=provisioned registration error;registration error;preparation error;provisioning error
	ErrorType ErrorType `json:"errorType,omitempty"`

	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Type of the server.
	Type string `json:"type,omitempty"`

	// Region contains the server location.
	Region Region `json:"type,omitempty"`

	// The hardware discovered to exist on the host.
	HardwareDetails *HardwareDetails `json:"hardware,omitempty"`

	// Information tracked by the provisioner.
	Provisioning ProvisionStatus `json:"provisioning"`

	// the last error message reported by the provisioning subsystem.
	ErrorMessage string `json:"errorMessage"`

	// indicator for whether or not the host is powered on.
	PoweredOn bool `json:"poweredOn"`

	// OperationHistory holds information about operations performed
	// on this host.
	OperationHistory OperationHistory `json:"operationHistory,omitempty"`

	// ErrorCount records how many times the host has encoutered an error since the last successful operation.
	// +kubebuilder:default:=0
	ErrorCount int `json:"errorCount"`
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

// ProvisionStatus holds the state information for a single target.
type ProvisionStatus struct {
	// An indiciator for what the provisioner is doing with the host.
	State ProvisioningState `json:"state"`

	// The machine's UUID from the underlying provisioning tool
	ID string `json:"ID"`

	// Image holds the details of the last image successfully
	// provisioned to the host.
	Image string `json:"image,omitempty"`

	// The RootDevicehints set by the user
	RootDeviceHints *RootDeviceHints `json:"rootDeviceHints,omitempty"`
}

// Capacity is a disk size in Bytes.
type Capacity int64

// Capacity multipliers.
const (
	Byte     Capacity = 1
	KibiByte          = Byte * 1024
	KiloByte          = Byte * 1000
	MebiByte          = KibiByte * 1024
	MegaByte          = KiloByte * 1000
	GibiByte          = MebiByte * 1024
	GigaByte          = MegaByte * 1000
	TebiByte          = GibiByte * 1024
	TeraByte          = GigaByte * 1000
)

// DiskType is a disk type, i.e. HDD, SSD, NVME.
type DiskType string

// DiskType constants.
const (
	HDD  DiskType = "HDD"
	SSD  DiskType = "SSD"
	NVME DiskType = "NVME"
)

// ClockSpeed is a clock speed in MHz
// +kubebuilder:validation:Format=double
type ClockSpeed string

// ClockSpeed multipliers.
const (
	MegaHertz ClockSpeed = "1.0"
	GigaHertz            = "1000"
)

// CPU describes one processor on the host.
type CPU struct {
	Arch           string     `json:"arch,omitempty"`
	Model          string     `json:"model,omitempty"`
	ClockMegahertz ClockSpeed `json:"clockMegahertz,omitempty"`
	Flags          []string   `json:"flags,omitempty"`
	Count          int        `json:"count,omitempty"`
}

// Storage describes one storage device (disk, SSD, etc.) on the host.
type Storage struct {
	// The Linux device name of the disk, e.g. "/dev/sda". Note that this
	// may not be stable across reboots.
	Name string `json:"name,omitempty"`

	// Device type, one of: HDD, SSD, NVME.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=HDD;SSD;NVME;
	Type DiskType `json:"type,omitempty"`

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
	SpeedGbps int `json:"speedGbps,omitempty"`
}

// HardwareDetails collects all of the information about hardware
// discovered on the host.
type HardwareDetails struct {
	RAMMebibytes int       `json:"ramMebibytes,omitempty"`
	NIC          []NIC     `json:"nics,omitempty"`
	Storage      []Storage `json:"storage,omitempty"`
	CPU          CPU       `json:"cpu,omitempty"`
	Hostname     string    `json:"hostname,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hbmh;hbmhost
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.operationalStatus",description="Operational status",priority=1
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.provisioning.state",description="Provisioning status"
// +kubebuilder:printcolumn:name="Consumer",type="string",JSONPath=".spec.consumerRef.name",description="Consumer using this host"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".status.type",description="The type of server",priority=1
// +kubebuilder:printcolumn:name="Online",type="string",JSONPath=".spec.online",description="Whether the host is online or not"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.errorType",description="Type of the most recent error"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of BaremetalHost"

// HetznerBareMetalHost is the Schema for the hetznerbaremetalhosts API.
type HetznerBareMetalHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HetznerBareMetalHostSpec   `json:"spec,omitempty"`
	Status HetznerBareMetalHostStatus `json:"status,omitempty"`
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
