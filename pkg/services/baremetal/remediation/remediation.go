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

// Package remediation implements functions to manage the lifecycle of baremetal remediation.
package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/host"
)

// Service defines struct with BareMetalRemediationScope to reconcile HetznerBareMetalRemediations.
type Service struct {
	scope *scope.BareMetalRemediationScope
}

// NewService outs a new service with BareMetalRemediationScope.
func NewService(scope *scope.BareMetalRemediationScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HetznerBareMetalRemediations.
func (s *Service) Reconcile(ctx context.Context) (reconcile.Result, error) {
	host, err := host.GetAssociatedHost(ctx, s.scope.Client, s.scope.BareMetalMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx,
				"exit remediation because host associated with hbmm no longer exists")
		}

		// retry
		err := fmt.Errorf("failed to find the unhealthy host (will retry): %w", err)
		record.Warn(s.scope.BareMetalRemediation, "FailedToFindHost", err.Error())
		return reconcile.Result{}, err
	}

	if host == nil {
		return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx,
			"exit remediation because hbmm has no host annotation")
	}

	if host.Spec.Status.HasFatalError() {
		return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx,
			fmt.Sprintf("exit remediation because host has error: %s: %s",
				host.Spec.Status.ErrorType,
				host.Spec.Status.ErrorMessage))
	}

	// if host is not provisioned, do not try to reboot server
	if host.Spec.Status.ProvisioningState != infrav1.StateProvisioned {
		return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx,
			fmt.Sprintf("exit remediation because host is not provisioned. Provisioning state: %s.",
				host.Spec.Status.ProvisioningState))
	}

	// host is in maintenance mode, do not try to reboot server
	if host.Spec.MaintenanceMode != nil && *host.Spec.MaintenanceMode {
		return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx,
			"exit remediation because host is in maintenance mode")
	}

	if s.scope.BareMetalRemediation.Spec.Strategy.Type != infrav1.RemediationTypeReboot {
		record.Warn(s.scope.BareMetalRemediation, "UnsupportedRemediationStrategy", "unsupported remediation strategy")
		return reconcile.Result{}, nil
	}

	// Skip remediation when the previous successful remediation is within the cooldown window.
	// Only evaluated on a fresh CR (Phase is empty) to avoid interrupting in-progress remediations.
	if s.scope.BareMetalRemediation.Status.Phase == "" {
		cooldown := s.scope.BareMetalRemediation.Spec.Strategy.EffectiveCooldown()
		if cooldown > 0 && s.scope.BareMetalMachine.Status.LastRemediatedAt != nil {
			since := time.Since(s.scope.BareMetalMachine.Status.LastRemediatedAt.Time)
			if since < cooldown {
				err := s.markRemediationSkipped(ctx,
					fmt.Sprintf("skipping reboot: last remediation completed %s ago (cooldown window: %s)",
						since.Round(time.Second), cooldown.Round(time.Second)))
				if err != nil {
					record.Warn(s.scope.BareMetalRemediation, "FailedSettingConditionOnMachine", err.Error())
					return reconcile.Result{}, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
				}
				return reconcile.Result{}, nil
			}
		}
	}

	// If no phase set, default to running
	if s.scope.BareMetalRemediation.Status.Phase == "" {
		s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseRunning
	}

	switch s.scope.BareMetalRemediation.Status.Phase {
	case infrav1.PhaseRunning:
		return s.handlePhaseRunning(ctx, host)
	case infrav1.PhaseWaiting:
		return s.handlePhaseWaiting(ctx, host)
	case infrav1.PhaseDeleting, infrav1.PhaseSucceeded:
		return reconcile.Result{}, nil
	default:
		return reconcile.Result{}, fmt.Errorf("internal error, unhandled BareMetalRemediation.Status.Phase: %v", s.scope.BareMetalRemediation.Status.Phase)
	}
}

func (s *Service) handlePhaseRunning(ctx context.Context, host *infrav1.HetznerBareMetalHost) (res reconcile.Result, err error) {
	// if host has not been remediated yet, do that now
	if s.scope.BareMetalRemediation.Status.LastRemediated == nil {
		if err := s.remediate(ctx, host); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed remediate host: %w", err)
		}
	}

	// if no retries are left, then change to phase waiting and return
	if !s.scope.HasRetriesLeft() {
		s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseWaiting
		return
	}

	nextRemediation := s.timeUntilNextRemediation(time.Now())

	if nextRemediation > 0 {
		// requeue until it is time for the remediation
		return reconcile.Result{RequeueAfter: nextRemediation}, nil
	}

	// remediate now
	if err := s.remediate(ctx, host); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed remediate host: %w", err)
	}

	return res, nil
}

func (s *Service) remediate(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	var err error

	patchHelper, err := v1beta1patch.NewHelper(host, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", host.Kind, host.Namespace, host.Name, err)
	}

	// add annotation to host so that it reboots
	host.Annotations, err = addRebootAnnotation(host.Annotations)
	if err != nil {
		record.Warn(s.scope.BareMetalRemediation, "FailedAddingRebootAnnotation", err.Error())
		return fmt.Errorf("failed to add reboot annotation: %w", err)
	}

	if err := patchHelper.Patch(ctx, host); err != nil {
		return fmt.Errorf("failed to patch: %s %s/%s %w", host.Kind, host.Namespace, host.Name, err)
	}

	record.Event(s.scope.BareMetalRemediation, "AnnotationAdded", "Reboot annotation is added to the BareMetalHost")

	// update status of BareMetalRemediation object
	now := metav1.Now()
	s.scope.BareMetalRemediation.Status.LastRemediated = &now
	s.scope.BareMetalRemediation.Status.RetryCount++

	return nil
}

func (s *Service) handlePhaseWaiting(ctx context.Context, host *infrav1.HetznerBareMetalHost) (res reconcile.Result, err error) {
	nextCheck := s.timeUntilNextRemediation(time.Now())

	if nextCheck > 0 {
		// Not yet time to stop remediation, requeue
		return reconcile.Result{RequeueAfter: nextCheck}, nil
	}

	capiMachine, err := util.GetOwnerMachine(ctx, s.scope.Client, s.scope.BareMetalRemediation.ObjectMeta)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("failed to get owner machine: %w", err)
	}
	if capiMachine != nil && conditions.IsTrue(capiMachine, clusterv1.MachineNodeHealthyCondition) {
		return reconcile.Result{}, s.markRemediationSucceeded(ctx, capiMachine,
			"reboot remediation succeeded: Node is healthy again")
	}

	// Reboots are exhausted and the node is still unhealthy. Retire also gets the
	// machine deleted (like the reuse path below), but via a permanent error on the
	// host, which additionally keeps the host out of the pool. See retireHost.
	if s.scope.BareMetalRemediation.Spec.Strategy.OnExhaustion == infrav1.OnExhaustionRetire {
		return reconcile.Result{}, s.retireHost(ctx, host)
	}

	return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx, "because retryLimit is reached and reboot timed out")
}

// retireHost sets a permanent error on the host so it leaves the cluster instead of
// being reused. The permanent error deletes the machine through the HasFatalError path,
// and skipHost keeps the host out of selection (the error is not cleared on deprovision)
// until a human removes the permanent-error annotation.
func (s *Service) retireHost(ctx context.Context, host *infrav1.HetznerBareMetalHost) error {
	patchHelper, err := v1beta1patch.NewHelper(host, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", host.Kind, host.Namespace, host.Name, err)
	}

	// RetryCount is the number of reboots attempted; it is 0 when retryLimit is 0
	// (retire without rebooting).
	retryCount := s.scope.BareMetalRemediation.Status.RetryCount
	reason := "retired by remediation: retryLimit is 0, node retired without a reboot attempt"
	if retryCount > 0 {
		reason = fmt.Sprintf("retired by remediation: node still unhealthy after %d failed reboot(s)", retryCount)
	}
	host.SetError(infrav1.PermanentError, reason)

	if err := patchHelper.Patch(ctx, host); err != nil {
		return fmt.Errorf("failed to patch: %s %s/%s %w", host.Kind, host.Namespace, host.Name, err)
	}

	record.Warn(s.scope.BareMetalRemediation, "HostRetired", reason)

	s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting
	return nil
}

// timeUntilNextRemediation checks if it is time to execute a next remediation step
// and returns seconds to next remediation time.
func (s *Service) timeUntilNextRemediation(now time.Time) time.Duration {
	timeout := s.scope.BareMetalRemediation.Spec.Strategy.Timeout.Duration
	// status is not updated yet
	if s.scope.BareMetalRemediation.Status.LastRemediated == nil {
		return timeout
	}

	if s.scope.BareMetalRemediation.Status.LastRemediated.Add(timeout).Before(now) {
		return time.Duration(0)
	}

	lastRemediated := now.Sub(s.scope.BareMetalRemediation.Status.LastRemediated.Time)
	nextRemediation := timeout - lastRemediated + time.Second
	return nextRemediation
}

// setOwnerRemediatedConditionToFailed sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck. This will make capi delete the capi and baremetal machine.
func (s *Service) setOwnerRemediatedConditionToFailed(ctx context.Context, msg string) error {
	capiMachine, err := util.GetOwnerMachine(ctx, s.scope.Client, s.scope.BareMetalRemediation.ObjectMeta)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			// Maybe a network error. Retry
			return fmt.Errorf("failed to get capi machine: %w", err)
		}
		record.Event(s.scope.BareMetalRemediation, "CapiMachineGone", "CAPI machine does not exist. Remediation will be stopped. Infra Machine will be deleted soon by GC.")

		// do not retry
		s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting
		return nil
	}

	// There is a capi-machine. Set condition.
	patchHelper, err := patch.NewHelper(capiMachine, s.scope.Client)
	if err != nil {
		// retry
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
	// moves control to CAPI machine controller. The owning controller will do
	// preflight checks and handles the Machine deletion.
	// Dual-write: deprecated v1beta1 list AND v1beta2 list. CAPI v1.13 MachineSet
	// reads from the v1beta2 list to trigger Machine deletion.
	deprecatedv1beta1conditions.MarkFalse(
		capiMachine,
		clusterv1.MachineOwnerRemediatedV1Beta1Condition,
		clusterv1.WaitingForRemediationV1Beta1Reason,
		clusterv1.ConditionSeverityWarning,
		"Remediation finished (machine will be deleted): %s", msg,
	)
	conditions.Set(capiMachine, metav1.Condition{
		Type:    clusterv1.MachineOwnerRemediatedCondition,
		Status:  metav1.ConditionFalse,
		Reason:  clusterv1.MachineOwnerRemediatedWaitingForRemediationReason,
		Message: fmt.Sprintf("Remediation finished (machine will be deleted): %s", msg),
	})

	if err := patchHelper.Patch(ctx, capiMachine); err != nil {
		// retry
		return fmt.Errorf("failed to patch: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	record.Event(s.scope.BareMetalRemediation, "ExitRemediation", msg)

	// do not retry
	s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting
	return nil
}

// markRemediationSucceeded updates three resources on success:
//   - CAPI Machine: clears the cluster.x-k8s.io/remediate-machine annotation
//     (CAPI MHC does not clear it for external remediation).
//   - HetznerBareMetalMachine: stamps LastRemediatedAt on its status for the
//     cooldown guard.
//   - HetznerBareMetalRemediation: moves the CR to PhaseSucceeded.
//
// MachineOwnerRemediatedCondition on the CAPI Machine is intentionally left
// untouched: it belongs to the Machine's owning controller
// (MachineSet/KCP/MachineDeployment), not to external remediation.
func (s *Service) markRemediationSucceeded(ctx context.Context, capiMachine *clusterv1.Machine, msg string) error {
	machinePatchHelper, err := patch.NewHelper(capiMachine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	delete(capiMachine.Annotations, clusterv1.RemediateMachineAnnotation)

	if err := machinePatchHelper.Patch(ctx, capiMachine); err != nil {
		return fmt.Errorf("failed to patch: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	bareMetalMachinePatchHelper, err := patch.NewHelper(s.scope.BareMetalMachine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper for baremetal machine: %w", err)
	}

	now := metav1.Now()
	s.scope.BareMetalMachine.Status.LastRemediatedAt = &now

	if err := bareMetalMachinePatchHelper.Patch(ctx, s.scope.BareMetalMachine); err != nil {
		return fmt.Errorf("failed to patch baremetal machine: %w", err)
	}

	record.Event(s.scope.BareMetalRemediation, "RemediationSucceeded", msg)

	s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseSucceeded
	return nil
}

// markRemediationSkipped escalates to deletion via MachineOwnerRemediated=False
// using RemediationCooldownTriggeredReason, so operators can distinguish a
// cooldown skip from a genuine failure.
func (s *Service) markRemediationSkipped(ctx context.Context, msg string) error {
	capiMachine, err := util.GetOwnerMachine(ctx, s.scope.Client, s.scope.BareMetalRemediation.ObjectMeta)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get capi machine: %w", err)
		}
		record.Event(s.scope.BareMetalRemediation, "CapiMachineGone", "CAPI machine does not exist. Remediation will be stopped. Infra Machine will be deleted soon by GC.")
		s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting
		return nil
	}

	patchHelper, err := patch.NewHelper(capiMachine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	// Dual-write: deprecated v1beta1 list AND v1beta2 list. CAPI v1.13 MachineSet
	// reads from the v1beta2 list to trigger Machine deletion.
	deprecatedv1beta1conditions.MarkFalse(
		capiMachine,
		clusterv1.MachineOwnerRemediatedV1Beta1Condition,
		infrav1.RemediationCooldownTriggeredReason,
		clusterv1.ConditionSeverityWarning,
		"Remediation cooldown active (machine will be deleted): %s", msg,
	)
	conditions.Set(capiMachine, metav1.Condition{
		Type:    clusterv1.MachineOwnerRemediatedCondition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.RemediationCooldownTriggeredReason,
		Message: fmt.Sprintf("Remediation cooldown active (machine will be deleted): %s", msg),
	})

	if err := patchHelper.Patch(ctx, capiMachine); err != nil {
		return fmt.Errorf("failed to patch: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	record.Event(s.scope.BareMetalRemediation, "RemediationSkipped", msg)

	s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting
	return nil
}

// addRebootAnnotation sets reboot annotation on unhealthy host.
func addRebootAnnotation(annotations map[string]string) (map[string]string, error) {
	rebootAnnotationArguments := infrav1.RebootAnnotationArguments{Type: infrav1.RebootTypeHardware}

	b, err := json.Marshal(rebootAnnotationArguments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reboot annotation arguments %+v: %w", rebootAnnotationArguments, err)
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[infrav1.RebootAnnotation] = string(b)
	return annotations, nil
}
