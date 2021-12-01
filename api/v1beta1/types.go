package v1beta1

// SSHKeySpec defines the SSHKey.
type SSHKeySpec struct {
	Name *string `json:"name,omitempty"`
	ID   *int    `json:"id,omitempty"`
}

// LoadBalancerAlgorithmType defines the Algorithm type.
//+kubebuilder:validation:Enum=round_robin;least_connections
type LoadBalancerAlgorithmType string

// HCloudMachineTypeSpec defines the HCloud Machine type.
type HCloudMachineTypeSpec string

// ResourceLifecycle configures the lifecycle of a resource.
type ResourceLifecycle string

// HCloudPlacementGroupSpec defines a PlacementGroup.
type HCloudPlacementGroupSpec struct {
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Enum=spread
	Type string `json:"type,omitempty"`
}

// HCloudPlacementGroupStatus returns the status of a Placementgroup.
type HCloudPlacementGroupStatus struct {
	ID     int    `json:"id,omitempty"`
	Server []int  `json:"server,omitempty"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
}
