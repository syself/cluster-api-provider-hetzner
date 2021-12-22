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
	"github.com/hetznercloud/hcloud-go/hcloud"
)

// LoadBalancerAlgorithmType defines the Algorithm type.
//+kubebuilder:validation:Enum=round_robin;least_connections
type LoadBalancerAlgorithmType string

const (

	// LoadBalancerAlgorithmTypeRoundRobin default for the Kubernetes Api Server loadbalancer.
	LoadBalancerAlgorithmTypeRoundRobin = LoadBalancerAlgorithmType("round_robin")

	// LoadBalancerAlgorithmTypeLeastConnections default for Loadbalancer.
	LoadBalancerAlgorithmTypeLeastConnections = LoadBalancerAlgorithmType("least_connections")
)

// HCloudAlgorithmType converts LoadBalancerAlgorithmType to hcloud type.
func (algorithmType *LoadBalancerAlgorithmType) HCloudAlgorithmType() hcloud.LoadBalancerAlgorithmType {
	switch *algorithmType {
	case LoadBalancerAlgorithmTypeLeastConnections:
		return hcloud.LoadBalancerAlgorithmTypeRoundRobin
	case LoadBalancerAlgorithmTypeRoundRobin:
		return hcloud.LoadBalancerAlgorithmTypeLeastConnections
	}
	return hcloud.LoadBalancerAlgorithmType("")
}

// HetznerSSHKeys defines the global SSHKeys HetznerCluster.
type HetznerSSHKeys struct {
	// +optional
	HCloud               []SSHKey     `json:"hcloud,omitempty"`
	RobotRescueSecretRef SSHSecretRef `json:"robotRescueSecretRef,omitempty"`
}

// SSHKey defines the SSHKey for HCloud.
type SSHKey struct {
	// Name of SSH key
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Fingerprint of SSH key - added by controller
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
	ID     int    `json:"id,omitempty"`
	Server []int  `json:"servers,omitempty"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
}

// HetznerSecretRef defines all the name of the secret and the relevant keys needed to access Hetzner API.
type HetznerSecretRef struct {
	Name string              `json:"name"`
	Key  HetznerSecretKeyRef `json:"key"`
}

// HetznerSecretKeyRef defines the key name of the HetznerSecret.
// Need to specify either HCloudToken or both HetznerRobotUser and HetznerRobotPassword.
type HetznerSecretKeyRef struct {
	// +optional
	HCloudToken string `json:"hcloudToken"`
	// +optional
	HetznerRobotUser string `json:"hetznerRobotUser"`
	// +optional
	HetznerRobotPassword string `json:"hetznerRobotPassword"`
}

// LoadBalancerSpec defines the desired state of the Control Plane Loadbalancer.
type LoadBalancerSpec struct {
	// +optional
	Name *string `json:"name,omitempty"`

	// Could be round_robin or least_connection. The default value is "round_robin".
	// +optional
	// +kubebuilder:validation:Enum=round_robin;least_connections
	// +kubebuilder:default=round_robin
	Algorithm LoadBalancerAlgorithmType `json:"algorithm,omitempty"`

	// Loadbalancer type
	// +optional
	// +kubebuilder:validation:Enum=lb11;lb21;lb31
	// +kubebuilder:default=lb11
	Type string `json:"type,omitempty"`

	// API Server port. It must be valid ports range (1-65535). If omitted, default value is 6443.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6443
	Port int `json:"port,omitempty"`

	// Defines how traffic will be routed from the Load Balancer to your target server.
	// +optional
	ExtraServices []LoadBalancerServiceSpec `json:"extraServices,omitempty"`

	// Region contains the name of the HCloud location the load balancer is running.
	Region Region `json:"region"`
}

// LoadBalancerServiceSpec defines a Loadbalancer Target.
type LoadBalancerServiceSpec struct {
	// Protocol specifies the supported Loadbalancer Protocol.
	// +kubebuilder:validation:Enum=http;https;tcp
	Protocol string `json:"protocol,omitempty"`

	// ListenPort, i.e. source port, defines the incoming port open on the loadbalancer.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort int `json:"listenPort,omitempty"`

	// DestinationPort defines the port on the server.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	DestinationPort int `json:"destinationPort,omitempty"`
}

// LoadBalancerStatus defines the obeserved state of the control plane loadbalancer.
type LoadBalancerStatus struct {
	ID         int    `json:"id,omitempty"`
	IPv4       string `json:"ipv4,omitempty"`
	IPv6       string `json:"ipv6,omitempty"`
	InternalIP string `json:"internalIP,omitempty"`
	Target     []int  `json:"targets,omitempty"`
	Protected  bool   `json:"protected,omitempty"`
}

// HCloudNetworkSpec defines the desired state of the HCloud Private Network.
type HCloudNetworkSpec struct {
	// Enabled defines whether the network should be enabled or not
	Enabled bool `json:"enabled"`

	// CIDRBlock defines the cidrBlock of the HCloud Network. A Subnet is required.
	// +kubebuilder:default="10.0.0.0/16"
	// +optional
	CIDRBlock string `json:"cidrBlock,omitempty"`

	// SubnetCIDRBlock defines the cidrBlock for the subnet of the HCloud Network.
	// +kubebuilder:default="10.0.0.0/24"
	// +optional
	SubnetCIDRBlock string `json:"subnetCidrBlock,omitempty"`

	// NetworkZone specifies the HCloud network zone of the private network.
	// +kubebuilder:validation:Enum=eu-central;us-east
	// +kubebuilder:default=eu-central
	// +optional
	NetworkZone HCloudNetworkZone `json:"networkZone,omitempty"`
}

// NetworkStatus defines the observed state of the HCloud Private Network.
type NetworkStatus struct {
	ID              int               `json:"id,omitempty"`
	Labels          map[string]string `json:"-"`
	AttachedServers []int             `json:"attachedServers,omitempty"`
}

// Region is a Hetzner Location
// +kubebuilder:validation:Enum=fsn1;hel1;nbg1;ash
type Region string

// HCloudNetworkZone describes the Network zone.
type HCloudNetworkZone string

// IsZero returns true if a private Network is set.
func (s *HCloudNetworkSpec) IsZero() bool {
	if len(s.CIDRBlock) > 0 {
		return false
	}
	if len(s.SubnetCIDRBlock) > 0 {
		return false
	}
	return true
}
