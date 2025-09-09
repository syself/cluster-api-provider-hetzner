/*
Copyright 2021 The Kubernetes Authors.

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
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// LoadBalancerAlgorithmType defines the Algorithm type.
// +kubebuilder:validation:Enum=round_robin;least_connections
type LoadBalancerAlgorithmType string

const (

	// LoadBalancerAlgorithmTypeRoundRobin default for the Kubernetes Api Server load balancer.
	LoadBalancerAlgorithmTypeRoundRobin = LoadBalancerAlgorithmType("round_robin")

	// LoadBalancerAlgorithmTypeLeastConnections default for load balancer.
	LoadBalancerAlgorithmTypeLeastConnections = LoadBalancerAlgorithmType("least_connections")
)

// LoadBalancerTargetType defines the target type.
// +kubebuilder:validation:Enum=server;ip
type LoadBalancerTargetType string

const (

	// LoadBalancerTargetTypeServer default for the Kubernetes Api Server load balancer.
	LoadBalancerTargetTypeServer = LoadBalancerTargetType("server")

	// LoadBalancerTargetTypeIP default for load balancer.
	LoadBalancerTargetTypeIP = LoadBalancerTargetType("ip")
)

// HCloudAlgorithmType converts LoadBalancerAlgorithmType to hcloud type.
func (algorithmType *LoadBalancerAlgorithmType) HCloudAlgorithmType() hcloud.LoadBalancerAlgorithmType {
	switch *algorithmType {
	case LoadBalancerAlgorithmTypeLeastConnections:
		return hcloud.LoadBalancerAlgorithmTypeLeastConnections
	case LoadBalancerAlgorithmTypeRoundRobin:
		return hcloud.LoadBalancerAlgorithmTypeRoundRobin
	}
	return hcloud.LoadBalancerAlgorithmType("")
}

// HetznerSSHKeys defines the global cluster-wide SSHKeys for HetznerCluster. It serves as the default for machines as well.
type HetznerSSHKeys struct {
	// Hcloud defines the SSH keys used for hcloud.
	// +optional
	HCloud []SSHKey `json:"hcloud,omitempty"`
	// RobotRescueSecretRef defines the reference to the secret where the SSH key for the rescue system is stored.
	RobotRescueSecretRef SSHSecretRef `json:"robotRescueSecretRef,omitempty"`
}

// SSHKey defines the SSHKey for HCloud.
type SSHKey struct {
	// Name defines the name of the SSH key.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Fingerprint defines the fingerprint of the SSH key - added by the controller.
	// +optional
	Fingerprint string `json:"fingerprint,omitempty"`
}

// HCloudMachineType defines the HCloud Machine type.
type HCloudMachineType string

// ResourceLifecycle configures the lifecycle of a resource.
type ResourceLifecycle string

// HCloudPlacementGroupSpec defines a PlacementGroup.
type HCloudPlacementGroupSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=spread
	// +kubebuilder:default=spread
	Type string `json:"type,omitempty"`
}

// HCloudPlacementGroupStatus returns the status of a Placementgroup.
type HCloudPlacementGroupStatus struct {
	ID     int64   `json:"id,omitempty"`
	Server []int64 `json:"servers,omitempty"`
	Name   string  `json:"name,omitempty"`
	Type   string  `json:"type,omitempty"`
}

// HetznerSecretRef defines all the names of the secret and the relevant keys needed to access Hetzner API.
type HetznerSecretRef struct {
	// Name defines the name of the secret.
	// +kubebuilder:default=hetzner
	Name string `json:"name"`
	// Key defines the keys that are used in the secret.
	// Need to specify either HCloudToken or both HetznerRobotUser and HetznerRobotPassword.
	Key HetznerSecretKeyRef `json:"key"`
}

// HetznerSecretKeyRef defines the key name of the HetznerSecret.
// Need to specify either HCloudToken or both HetznerRobotUser and HetznerRobotPassword.
type HetznerSecretKeyRef struct {
	// HCloudToken defines the name of the key where the token for the Hetzner Cloud API is stored.
	// +optional
	// +kubebuilder:default=hcloud-token
	HCloudToken string `json:"hcloudToken"`
	// HetznerRobotUser defines the name of the key where the username for the Hetzner Robot API is stored.
	// +optional
	// +kubebuilder:default=hetzner-robot-user
	HetznerRobotUser string `json:"hetznerRobotUser"`
	// HetznerRobotPassword defines the name of the key where the password for the Hetzner Robot API is stored.
	// +optional
	// +kubebuilder:default=hetzner-robot-password
	HetznerRobotPassword string `json:"hetznerRobotPassword"`
	// SSHKey defines the name of the ssh key.
	// +optional
	// +kubebuilder:default=hcloud-ssh-key-name
	SSHKey string `json:"sshKey"`
}

// PublicNetworkSpec contains specs about the public network spec of an HCloud server.
type PublicNetworkSpec struct {
	// EnableIPv4 defines whether server has IPv4 address enabled.
	// As Hetzner load balancers require an IPv4 address, this setting will be ignored and set to true if there is no private net.
	// +optional
	// +kubebuilder:default=true
	EnableIPv4 bool `json:"enableIPv4"`
	// EnableIPv6 defines whether server has IPv6 addresses enabled.
	// +optional
	// +kubebuilder:default=true
	EnableIPv6 bool `json:"enableIPv6"`
}

// LoadBalancerSpec defines the desired state of the Control Plane load balancer.
type LoadBalancerSpec struct {
	// Enabled specifies if a load balancer should be created.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Name defines the name of the load balancer. It can be specified in order to use an existing load balancer.
	// +optional
	Name *string `json:"name,omitempty"`

	// Algorithm defines the type of load balancer algorithm. It could be round_robin or least_connection. The default value is "round_robin".
	// +optional
	// +kubebuilder:validation:Enum=round_robin;least_connections
	// +kubebuilder:default=round_robin
	Algorithm LoadBalancerAlgorithmType `json:"algorithm,omitempty"`

	// Type defines the type of load balancer. It could be one of lb11, lb21, or lb31.
	// +optional
	// +kubebuilder:validation:Enum=lb11;lb21;lb31
	// +kubebuilder:default=lb11
	Type string `json:"type,omitempty"`

	// Port defines the API Server port. It must be a valid port range (1-65535). If omitted, the default value is 6443.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6443
	Port int `json:"port,omitempty"`

	// ExtraServices defines how traffic will be routed from the load balancer to your target server.
	// +optional
	ExtraServices []LoadBalancerServiceSpec `json:"extraServices,omitempty"`

	// Region contains the name of the HCloud location where the load balancer is running.
	Region Region `json:"region,omitempty"`
}

// LoadBalancerServiceSpec defines a load balancer Target.
type LoadBalancerServiceSpec struct {
	// Protocol specifies the supported load balancer Protocol. It could be one of the https, http, or tcp.
	// +kubebuilder:validation:Enum=http;https;tcp
	Protocol string `json:"protocol,omitempty"`

	// ListenPort, i.e. source port, defines the incoming port open on the load balancer. It must be a valid port range (1-65535).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort int `json:"listenPort,omitempty"`

	// DestinationPort defines the port on the server. It must be a valid port range (1-65535).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	DestinationPort int `json:"destinationPort,omitempty"`
}

// LoadBalancerStatus defines the observed state of the control plane load balancer.
type LoadBalancerStatus struct {
	ID         int64                `json:"id,omitempty"`
	IPv4       string               `json:"ipv4,omitempty"`
	IPv6       string               `json:"ipv6,omitempty"`
	InternalIP string               `json:"internalIP,omitempty"`
	Target     []LoadBalancerTarget `json:"targets,omitempty"`
	Protected  bool                 `json:"protected,omitempty"`
}

// LoadBalancerTarget defines the target of a load balancer.
type LoadBalancerTarget struct {
	Type     LoadBalancerTargetType `json:"type"`
	ServerID int64                  `json:"serverID,omitempty"`
	IP       string                 `json:"ip,omitempty"`
}

// HCloudNetworkSpec defines the desired state of the HCloud Private Network.
type HCloudNetworkSpec struct {
	// Enabled defines whether the network should be enabled or not.
	Enabled bool `json:"enabled"`

	// CIDRBlock defines the cidrBlock of the HCloud Network. If omitted, default "10.0.0.0/16" will be used.
	// +kubebuilder:default="10.0.0.0/16"
	// +optional
	CIDRBlock string `json:"cidrBlock,omitempty"`

	// SubnetCIDRBlock defines the cidrBlock for the subnet of the HCloud Network.
	// Note: A subnet is required.
	// +kubebuilder:default="10.0.0.0/24"
	// +optional
	SubnetCIDRBlock string `json:"subnetCidrBlock,omitempty"`

	// NetworkZone specifies the HCloud network zone of the private network.
	// The zones must be one of eu-central, us-east, or us-west. The default is eu-central.
	// +kubebuilder:validation:Enum=eu-central;us-east;us-west;ap-southeast
	// +kubebuilder:default=eu-central
	// +optional
	NetworkZone HCloudNetworkZone `json:"networkZone,omitempty"`
}

// NetworkStatus defines the observed state of the HCloud Private Network.
type NetworkStatus struct {
	ID              int64             `json:"id,omitempty"`
	Labels          map[string]string `json:"-"`
	AttachedServers []int64           `json:"attachedServers,omitempty"`
}

// Region is a Hetzner Location.
// +kubebuilder:validation:Enum=fsn1;hel1;nbg1;ash;hil;sin
type Region string

// HCloudNetworkZone describes the Network zone.
type HCloudNetworkZone string

// IsZero returns true if a private Network is set.
func (s *HCloudNetworkSpec) IsZero() bool {
	if s.CIDRBlock != "" {
		return false
	}
	if s.SubnetCIDRBlock != "" {
		return false
	}
	return true
}

// HCloudBootState defines the boot state of an HCloud server during provisioning.
type HCloudBootState string

const (
	// HCloudBootStateUnset is the initial state when the boot state has not been set yet.
	HCloudBootStateUnset HCloudBootState = ""

	// HCloudBootStateWaitForPreRescueOSThenEnableRescueSystem indicates that the controller waits for PreRescueOS.
	// When it is available, then the rescue system gets enabled.
	HCloudBootStateWaitForPreRescueOSThenEnableRescueSystem HCloudBootState = "WaitForPreRescueOSThenEnableRescueSystem"

	// HCloudBootStateWaitForRescueEnabledThenRebootToRescue indicates that the controller waits for the rescue system enabled. Then the server gets booted into the rescue system.
	HCloudBootStateWaitForRescueEnabledThenRebootToRescue HCloudBootState = "WaitForRescueEnabledThenRebootToRescue"

	// HCloudBootStateWaitForRescueRunningThenInstallImage indicates that the node image is currently being
	// installed.
	HCloudBootStateWaitForRescueRunningThenInstallImage HCloudBootState = "WaitForRescueRunningThenInstallImage"

	HCloudBootStateWaitForRebootAfterInstallImageThenBootToRealOS HCloudBootState = "WaitForRebootAfterInstallImageThenBootToRealOS"

	// HCloudBootStateBootToRealOS indicates that the server is booting the operating system.
	HCloudBootStateBootToRealOS HCloudBootState = "BootToRealOS"

	// HCloudBootStateOperatingSystemRunning indicates that the server is successfully running.
	HCloudBootStateOperatingSystemRunning HCloudBootState = "OperatingSystemRunning"
)
