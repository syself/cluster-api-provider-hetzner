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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
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

	// If no phase set, default to running
	if s.scope.BareMetalRemediation.Status.Phase == "" {
		s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseRunning
	}

	switch s.scope.BareMetalRemediation.Status.Phase {
	case infrav1.PhaseRunning:
		return s.handlePhaseRunning(ctx, host)
	case infrav1.PhaseWaiting:
		return s.handlePhaseWaiting(ctx)
	case infrav1.PhaseDeleting:
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

	patchHelper, err := patch.NewHelper(host, s.scope.Client)
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

func (s *Service) handlePhaseWaiting(ctx context.Context) (res reconcile.Result, err error) {
	nextCheck := s.timeUntilNextRemediation(time.Now())

	if nextCheck > 0 {
		// Not yet time to stop remediation, requeue
		return reconcile.Result{RequeueAfter: nextCheck}, nil
	}

	return reconcile.Result{}, s.setOwnerRemediatedConditionToFailed(ctx, "because retryLimit is reached and reboot timed out")
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
	conditions.MarkFalse(
		capiMachine,
		clusterv1.MachineOwnerRemediatedCondition,
		clusterv1.WaitingForRemediationReason,
		clusterv1.ConditionSeverityWarning,
		"Remediation finished (machine will be deleted): %s", msg,
	)

	if err := patchHelper.Patch(ctx, capiMachine); err != nil {
		// retry
		return fmt.Errorf("failed to patch: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	record.Event(s.scope.BareMetalRemediation, "ExitRemediation", msg)

	// do not retry
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
