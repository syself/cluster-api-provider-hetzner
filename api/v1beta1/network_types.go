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

// HCloudNetworkSpec defines the desired state of the HCloud Private Network.
type HCloudNetworkSpec struct {
	NetworkEnabled bool `json:"enabled"`

	// Defines the cidrBlock of the HCloud Network. A Subnet is required.
	// +kubebuilder:default="10.0.0.0/16"
	CIDRBlock string `json:"cidrBlock,omitempty"`

	// +kubebuilder:default="10.0.0.0/24"
	SubnetCIDRBlock string `json:"subnetCidrBlock,omitempty"`

	// +kubebuilder:default=eu-central
	NetworkZone HCloudNetworkZone `json:"networkZone,omitempty"`
}

// NetworkStatus defines the observed state of the HCloud Private Network.
type NetworkStatus struct {
	ID             int               `json:"id,omitempty"`
	Labels         map[string]string `json:"-"`
	AttachedServer []int             `json:"attachedServer,omitempty"`
}

// Region is a Hetzner Location
// +kubebuilder:validation:Enum=fsn1;hel1;nbg1;ash
type Region string

// HCloudNetworkZone describes the Network zone.
type HCloudNetworkZone string

// IsZero return if a private Network is set or not.
func (s *HCloudNetworkSpec) IsZero() bool {
	if len(s.CIDRBlock) > 0 {
		return false
	}
	if len(s.SubnetCIDRBlock) > 0 {
		return false
	}
	return true
}

// LoadBalancerTargetSpec defines a Loadbalancer Target.
type LoadBalancerTargetSpec struct {
	// Protocol specifies the supported Loadbalancer Protocol.
	// +optional
	// +kubebuilder:validation:Enum=http;https;tcp
	Protocol string `json:"protocol,omitempty"`

	// Equal Source port, defines the incoming port open on the loadbalancer
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort int `json:"listenPort,omitempty"`

	// Defines the port on the server
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	DestinationPort int `json:"destinationPort,omitempty"`
}
