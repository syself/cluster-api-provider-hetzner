/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

// MachineInitializationStatus holds provisioning signals consumed by CAPI for Machines.
// Fields and json tags must follow the contract at
// https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-machine.html?highlight=Initialization#inframachine-initialization-completed
// (status.initialization.provisioned).
type MachineInitializationStatus struct {
	// Provisioned is true when the infrastructure provider reports the machine is fully provisioned.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}
