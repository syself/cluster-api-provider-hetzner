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

package v1beta2

import (
	"time"
)

// DefaultRemediationCooldown is the cooldown applied when the strategy does
// not configure one. The kubebuilder default populates the field at admission;
// this constant is the fallback for CRs that predate the default.
const DefaultRemediationCooldown = 30 * time.Minute

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

	// PhaseSucceeded represents the state where remediation brought the Node back to a healthy state
	// and the unhealthy Machine is kept.
	PhaseSucceeded = "Succeeded"
)

// RemediationStrategy describes how to remediate machines.
type RemediationStrategy struct {
	// Type represents the type of the remediation strategy. At the moment, only "Reboot" is supported.
	// +kubebuilder:default=Reboot
	// +optional
	Type RemediationType `json:"type,omitempty"`

	// RetryLimit sets the maximum number of remediation retries. Zero retries if not set.
	// +kubebuilder:validation:Minimum=0
	// +optional
	RetryLimit *int32 `json:"retryLimit,omitempty"`

	// TimeoutSeconds sets the timeout, in seconds, between remediation retries.
	// +kubebuilder:validation:Minimum=0
	TimeoutSeconds int32 `json:"timeoutSeconds"`

	// CooldownSeconds is the minimum time, in seconds, between successive
	// remediations of the same machine. If a new remediation starts within this
	// window of the previous successful one, the reboot is skipped and the
	// machine is deleted. An explicit 0 disables the guard and is distinct from an
	// absent value, which defaults to 1800 (30m).
	// +kubebuilder:default=1800
	// +kubebuilder:validation:Minimum=0
	// +optional
	CooldownSeconds *int32 `json:"cooldownSeconds,omitempty"`
}

// EffectiveCooldown returns the configured cooldown, or DefaultRemediationCooldown
// when the strategy or cooldownSeconds is unset. A negative value is clamped to the
// default as a defense-in-depth against admission bypass.
func (s *RemediationStrategy) EffectiveCooldown() time.Duration {
	if s == nil || s.CooldownSeconds == nil || *s.CooldownSeconds < 0 {
		return DefaultRemediationCooldown
	}
	return time.Duration(*s.CooldownSeconds) * time.Second
}
