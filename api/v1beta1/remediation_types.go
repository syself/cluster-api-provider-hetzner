/*
Copyright 2023 The Kubernetes Authors.

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RemediationType defines the type of remediation.
type RemediationType string

const (
	// RemediationTypeReboot sets RemediationType to Reboot.
	RemediationTypeReboot RemediationType = "Reboot"
)

const (
	// PhaseRunning represents the running state during remediation.
	PhaseRunning = "Running"

	// PhaseWaiting represents the state during remediation when the controller has done its job but still waiting for the result of the last remediation step.
	PhaseWaiting = "Waiting"

	// PhaseDeleting represents the state where host remediation has failed and the controller is deleting the unhealthy Machine object from the cluster.
	PhaseDeleting = "Deleting machine"
)

// RemediationStrategy describes how to remediate machines.
type RemediationStrategy struct {
	// Type represents the type of the remediation strategy. At the moment, only "Reboot" is supported.
	// +kubebuilder:default=Reboot
	// +optional
	Type RemediationType `json:"type,omitempty"`

	// RetryLimit sets the maximum number of remediation retries. Zero retries if not set.
	// +optional
	RetryLimit int `json:"retryLimit,omitempty"`

	// Timeout sets the timeout between remediation retries. It should be of the form "10m", or "40s".
	Timeout *metav1.Duration `json:"timeout"`
}
