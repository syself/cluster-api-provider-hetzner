/*
Copyright 2022 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// RemediationFinalizer allows HetznerBareMetalRemediationReconciler to clean up resources associated with HetznerBareMetalRemediation before
	// removing it from the apiserver.
	RemediationFinalizer = "hetznerbaremetalremediation.infrastructure.cluster.x-k8s.io"

	// RebootAnnotation indicates that a bare metal host object should be rebooted.
	RebootAnnotation = "capi.syself.com/reboot"

	// PermanentErrorAnnotation indicates that the bare metal host has an error which needs to be resolved manually.
	// After the permanent error the annotation got removed (usually by a human), the controller removes
	// ErrorType, ErrorCount and ErrorMessages, so that the hbmh will be usable again.
	PermanentErrorAnnotation = "capi.syself.com/permanent-error"
)

// HetznerBareMetalRemediationSpec defines the desired state of HetznerBareMetalRemediation.
type HetznerBareMetalRemediationSpec struct {
	// Strategy field defines the remediation strategy to be applied.
	Strategy *RemediationStrategy `json:"strategy,omitempty"`
}

// HetznerBareMetalRemediationStatus defines the observed state of HetznerBareMetalRemediation.
type HetznerBareMetalRemediationStatus struct {
	// Phase represents the current phase of machine remediation.
	// E.g. Pending, Running, Done etc.
	// +optional
	Phase string `json:"phase,omitempty"`

	// RetryCount can be used as a counter during the remediation.
	// Field can hold number of reboots etc.
	// +optional
	RetryCount int `json:"retryCount,omitempty"`

	// LastRemediated identifies when the host was last remediated
	// +optional
	LastRemediated *metav1.Time `json:"lastRemediated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalremediations,scope=Namespaced,categories=cluster-api,shortName=hbr
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=".spec.strategy.type",description="Type of the remediation strategy"
// +kubebuilder:printcolumn:name="Retry limit",type=string,JSONPath=".spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the host"
// +kubebuilder:printcolumn:name="Timeout",type=string,JSONPath=".spec.strategy.timeout",description="Timeout for the remediation"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="Phase of the remediation"
// +kubebuilder:printcolumn:name="Last Remediated",type=string,JSONPath=".status.lastRemediated",description="Timestamp of the last remediation attempt"
// +kubebuilder:printcolumn:name="Retry count",type=string,JSONPath=".status.retryCount",description="How many times remediation controller has tried to remediate the node"

// HetznerBareMetalRemediation is the Schema for the hetznerbaremetalremediations API.
type HetznerBareMetalRemediation struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Spec HetznerBareMetalRemediationSpec `json:"spec,omitempty"`
	// +optional
	Status HetznerBareMetalRemediationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HetznerBareMetalRemediationList contains a list of HetznerBareMetalRemediation.
type HetznerBareMetalRemediationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HetznerBareMetalRemediation `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &HetznerBareMetalRemediation{}, &HetznerBareMetalRemediationList{})
}
