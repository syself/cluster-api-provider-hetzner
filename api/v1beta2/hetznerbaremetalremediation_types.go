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

package v1beta2

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// RebootAnnotation indicates that a bare metal host object should be rebooted.
	RebootAnnotation = "capi.syself.com/reboot"

	// PermanentErrorAnnotation indicates that the bare metal host has an error which needs to be resolved manually.
	// After the annotation is removed (usually by a human), the controller clears the error, so that the
	// HetznerBareMetalHost is usable again.
	PermanentErrorAnnotation = "capi.syself.com/permanent-error"
)

// HetznerBareMetalRemediationSpec defines the desired state of HetznerBareMetalRemediation.
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalRemediationSpec struct {
	// strategy field defines the remediation strategy to be applied.
	// +optional
	Strategy BareMetalRemediationStrategy `json:"strategy,omitempty,omitzero"`
}

// OnExhaustionAction defines what remediation does when the retries run out and
// the node is still unhealthy.
type OnExhaustionAction string

const (
	// OnExhaustionReuse deletes the machine and frees the host back into the
	// selectable pool, so it can be provisioned again. This is the behavior when
	// onExhaustion is unset, and the right choice for temporary problems.
	OnExhaustionReuse OnExhaustionAction = "Reuse"

	// OnExhaustionRetire sets a permanent error on the host, which deletes the
	// machine and keeps the host out of the pool until a human removes the
	// capi.syself.com/permanent-error annotation. Use this for real hardware
	// failures, where a reboot never helps.
	OnExhaustionRetire OnExhaustionAction = "Retire"
)

// BareMetalRemediationStrategy describes how to remediate bare metal machines.
// It reuses the shared RemediationStrategy fields and adds bare-metal-only options.
// Conversion is hand-written (the type is tagged +k8s:conversion-gen=false) because the
// embedded RemediationStrategy uses hand-written conversion.
// +k8s:conversion-gen=false
type BareMetalRemediationStrategy struct {
	// The shared remediation fields (type, retryLimit, timeoutSeconds, cooldownSeconds).
	RemediationStrategy `json:",inline"`

	// onExhaustion selects what happens when remediation runs out of retries and
	// the node is still unhealthy. Reuse deletes the machine and frees the host to be
	// provisioned again. Note: When unset it behaves like Reuse. Retire sets a permanent
	// error on the host, which deletes the machine and keeps the host out of the pool
	// until a human removes the capi.syself.com/permanent-error annotation.
	// +kubebuilder:validation:Enum=Reuse;Retire
	// +optional
	OnExhaustion OnExhaustionAction `json:"onExhaustion,omitempty"`
}

// IsDefined returns true if the BareMetalRemediationStrategy is set.
func (s *BareMetalRemediationStrategy) IsDefined() bool {
	if s == nil {
		return false
	}
	return !reflect.DeepEqual(s, &BareMetalRemediationStrategy{})
}

// HetznerBareMetalRemediationStatus defines the observed state of HetznerBareMetalRemediation.
// +kubebuilder:validation:MinProperties=1
type HetznerBareMetalRemediationStatus struct {
	// phase represents the current phase of machine remediation.
	// One of Running, Waiting, Deleting machine or Succeeded.
	// +optional
	// +kubebuilder:validation:Enum=Running;Waiting;"Deleting machine";Succeeded
	Phase string `json:"phase,omitempty"`

	// retryCount records how many times the remediation controller has tried to
	// remediate the node, for example the number of reboots.
	// +optional
	RetryCount *int32 `json:"retryCount,omitempty"`

	// lastRemediated identifies when the host was last remediated.
	// A zero value is treated as absent.
	// +optional
	LastRemediated metav1.Time `json:"lastRemediated,omitempty,omitzero"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=hetznerbaremetalremediations,scope=Namespaced,categories=cluster-api,shortName=hbr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="Phase of the remediation"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of the remediation"
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=".spec.strategy.type",description="Type of the remediation strategy",priority=1
// +kubebuilder:printcolumn:name="Retry limit",type=integer,JSONPath=".spec.strategy.retryLimit",description="How many times remediation controller should attempt to remediate the host",priority=1
// +kubebuilder:printcolumn:name="Timeout",type=integer,JSONPath=".spec.strategy.timeoutSeconds",description="Timeout for the remediation",priority=1
// +kubebuilder:printcolumn:name="Last Remediated",type=string,JSONPath=".status.lastRemediated",description="Timestamp of the last remediation attempt",priority=1
// +kubebuilder:printcolumn:name="Retry count",type=integer,JSONPath=".status.retryCount",description="How many times remediation controller has tried to remediate the node",priority=1

// HetznerBareMetalRemediation is the Schema for the hetznerbaremetalremediations API.
type HetznerBareMetalRemediation struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of HetznerBareMetalRemediation. It is empty when the template that
	// created the remediation has no strategy. The controller then does nothing and records an event.
	// +optional
	Spec HetznerBareMetalRemediationSpec `json:"spec,omitempty,omitzero"`
	// status is the observed state of HetznerBareMetalRemediation.
	// +optional
	Status HetznerBareMetalRemediationStatus `json:"status,omitempty,omitzero"`
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
