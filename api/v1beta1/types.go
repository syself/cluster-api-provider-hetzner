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
	Server []int  `json:"servers,omitempty"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
}
