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

// Package remediation implements functions to manage the lifecycle of hcloud remediation.
package remediation

import (
	"context"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
)

// Service defines struct with machine scope to reconcile HCloudRemediation.
type Service struct {
	scope *scope.HCloudRemediationScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.HCloudRemediationScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HCloudRemediation.
func (s *Service) Reconcile(ctx context.Context) (reconcile.Result, error) {
	if s.scope.HCloudMachine.Status.BootState != infrav1.HCloudBootStateOperatingSystemRunning {
		err := s.setOwnerRemediatedConditionToFailed(ctx,
			fmt.Sprintf("exit remediation because infra machine is in BootState %s (no need to try a reboot)", s.scope.HCloudMachine.Status.BootState))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("setOwnerRemediatedConditionToFailed failed: %w", err)
		}
		return reconcile.Result{}, nil
	}

	var server *hcloud.Server
	if s.scope.HCloudMachine.Spec.ProviderID != nil {
		var err error
		server, err = s.findServer(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to find the server of unhealthy machine: %w", err)
		}
	}

	// stop remediation if server does not exist or ProviderID is nil (in this case the server
	// cannot exist).
	if server == nil {
		msg := "ProviderID is not set"
		if s.scope.HCloudMachine.Spec.ProviderID != nil {
			msg = fmt.Sprintf("No server found via hcloud api for providerID %q",
				*s.scope.HCloudMachine.Spec.ProviderID)
		}

		if err := s.setOwnerRemediatedConditionToFailed(ctx, msg); err != nil {
			record.Warn(s.scope.HCloudRemediation, "FailedSettingConditionOnMachine", err.Error())
			return reconcile.Result{}, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
		}
		return reconcile.Result{}, nil
	}

	if s.scope.HCloudRemediation.Spec.Strategy == nil {
		s.scope.Info("remediation strategy is nil")
		record.Warn(s.scope.HCloudRemediation, "UnsupportedRemdiationStrategy",
			"remediation strategy is nil")
		return reconcile.Result{}, nil
	}

	remediationType := s.scope.HCloudRemediation.Spec.Strategy.Type

	if remediationType != infrav1.RemediationTypeReboot {
		s.scope.Info("unsupported remediation strategy")
		record.Warnf(s.scope.HCloudRemediation, "UnsupportedRemdiationStrategy", "remediation strategy %q is unsupported", remediationType)
		return reconcile.Result{}, nil
	}

	// Skip remediation when the previous successful remediation is within the cooldown window.
	// Only evaluated on a fresh CR (Phase is empty) to avoid interrupting in-progress remediations.
	if s.scope.HCloudRemediation.Status.Phase == "" {
		cooldown := s.scope.HCloudRemediation.Spec.Strategy.EffectiveCooldown()
		if cooldown > 0 && s.scope.HCloudMachine.Status.LastRemediatedAt != nil {
			since := time.Since(s.scope.HCloudMachine.Status.LastRemediatedAt.Time)
			if since < cooldown {
				err := s.markRemediationSkipped(ctx,
					fmt.Sprintf("skipping reboot: last remediation completed %s ago (cooldown window: %s)",
						since.Round(time.Second), cooldown.Round(time.Second)))
				if err != nil {
					record.Warn(s.scope.HCloudRemediation, "FailedSettingConditionOnMachine", err.Error())
					return reconcile.Result{}, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
				}
				return reconcile.Result{}, nil
			}
		}
	}

	// If no phase set, default to running
	if s.scope.HCloudRemediation.Status.Phase == "" {
		s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseRunning
	}

	switch s.scope.HCloudRemediation.Status.Phase {
	case infrav1.PhaseRunning:
		return s.handlePhaseRunning(ctx, server)
	case infrav1.PhaseWaiting:
		return s.handlePhaseWaiting(ctx)
	}

	return reconcile.Result{}, nil
}

func (s *Service) handlePhaseRunning(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	now := metav1.Now()

	// if server has never been remediated, then do that now
	if s.scope.HCloudRemediation.Status.LastRemediated == nil {
		if err := s.scope.HCloudClient.RebootServer(ctx, server); err != nil {
			hcloudutil.HandleRateLimitExceededV1Beta1(s.scope.HCloudMachine, err, "RebootServer")
			record.Warn(s.scope.HCloudRemediation, "FailedRebootServer", err.Error())
			return reconcile.Result{}, fmt.Errorf("failed to reboot server %s with ID %d: %w", server.Name, server.ID, err)
		}
		record.Event(s.scope.HCloudRemediation, "ServerRebooted", "Server has been rebooted")

		s.scope.HCloudRemediation.Status.LastRemediated = &now
		s.scope.HCloudRemediation.Status.RetryCount++
	}

	retryLimit := s.scope.HCloudRemediation.Spec.Strategy.RetryLimit
	retryCount := s.scope.HCloudRemediation.Status.RetryCount

	// check whether retry limit has been reached
	if retryLimit == 0 || retryCount >= retryLimit {
		s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseWaiting
	}

	// check when next remediation should be scheduled
	nextRemediation := s.timeUntilNextRemediation(now.Time)

	if nextRemediation > 0 {
		// Not yet time to remediate, requeue
		return reconcile.Result{RequeueAfter: nextRemediation}, nil
	}

	// remediate now
	if err := s.scope.HCloudClient.RebootServer(ctx, server); err != nil {
		hcloudutil.HandleRateLimitExceededV1Beta1(s.scope.HCloudMachine, err, "RebootServer")
		record.Warn(s.scope.HCloudRemediation, "FailedRebootServer", err.Error())
		return reconcile.Result{}, fmt.Errorf("failed to reboot server %s with ID %d: %w", server.Name, server.ID, err)
	}
	record.Event(s.scope.HCloudRemediation, "ServerRebooted", "Server has been rebooted")

	s.scope.HCloudRemediation.Status.LastRemediated = &now
	s.scope.HCloudRemediation.Status.RetryCount++

	return res, nil
}

func (s *Service) handlePhaseWaiting(ctx context.Context) (reconcile.Result, error) {
	nextCheck := s.timeUntilNextRemediation(time.Now())

	if nextCheck > 0 {
		// Not yet time to stop remediation, requeue
		return reconcile.Result{RequeueAfter: nextCheck}, nil
	}

	// If the Node is healthy again, the reboot worked. Finish remediation
	// without touching MachineOwnerRemediatedCondition: that condition belongs
	// to the Machine's owning controller (MachineSet/KCP/MachineDeployment),
	// not to external remediation. Leaving it unset keeps the machine alive.
	if conditions.IsTrue(s.scope.Machine, clusterv1.MachineNodeHealthyCondition) {
		if err := s.markRemediationSucceeded(ctx,
			"reboot remediation succeeded: Node is healthy again"); err != nil {
			record.Warn(s.scope.HCloudRemediation, "FailedFinishingRemediation", err.Error())
			return reconcile.Result{}, fmt.Errorf("failed to finish remediation: %w", err)
		}
		return reconcile.Result{}, nil
	}

	err := s.setOwnerRemediatedConditionToFailed(ctx,
		"exit remediation because because retryLimit is reached and reboot timed out")
	if err != nil {
		record.Warn(s.scope.HCloudRemediation, "FailedSettingConditionOnMachine", err.Error())
		return reconcile.Result{}, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
	}

	// do not reconcile again.
	return reconcile.Result{}, nil
}

func (s *Service) findServer(ctx context.Context) (*hcloud.Server, error) {
	serverID, err := s.scope.ServerIDFromProviderID()
	if err != nil {
		return nil, fmt.Errorf("failed to get serverID from providerID: %w", err)
	}

	server, err := s.scope.HCloudClient.GetServer(ctx, serverID)
	if err != nil {
		hcloudutil.HandleRateLimitExceededV1Beta1(s.scope.HCloudMachine, err, "GetServer")
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	return server, nil
}

// setOwnerRemediatedConditionToFailed sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck. This will make capi delete the capi and hcloud machine.
func (s *Service) setOwnerRemediatedConditionToFailed(ctx context.Context, msg string) error {
	patchHelper, err := patch.NewHelper(s.scope.Machine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %w", err)
	}

	// Move control to CAPI machine controller. CAPI will delete the machine.
	// CAPI v1.13 MachineSet reads MachineOwnerRemediated from the v1beta2
	// conditions list, so we must write to BOTH lists: the deprecated v1beta1
	// list (legacy consumers) and the v1beta2 list (current MachineSet filter).
	deprecatedv1beta1conditions.MarkFalse(
		s.scope.Machine,
		clusterv1.MachineOwnerRemediatedV1Beta1Condition,
		clusterv1.WaitingForRemediationV1Beta1Reason,
		clusterv1.ConditionSeverityWarning,
		"Remediation finished (machine will be deleted): %s", msg,
	)
	conditions.Set(s.scope.Machine, metav1.Condition{
		Type:    clusterv1.MachineOwnerRemediatedCondition,
		Status:  metav1.ConditionFalse,
		Reason:  clusterv1.MachineOwnerRemediatedWaitingForRemediationReason,
		Message: fmt.Sprintf("Remediation finished (machine will be deleted): %s", msg),
	})

	if err := patchHelper.Patch(ctx, s.scope.Machine); err != nil {
		return fmt.Errorf("failed to patch: %w", err)
	}

	record.Event(s.scope.HCloudRemediation, "ExitRemediation", msg)

	s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseDeleting
	return nil
}

// markRemediationSucceeded updates three resources on success:
//   - CAPI Machine: clears the cluster.x-k8s.io/remediate-machine annotation
//     (CAPI MHC does not clear it for external remediation).
//   - HCloudMachine: stamps LastRemediatedAt on its status for the cooldown
//     guard.
//   - HCloudRemediation: moves the CR to PhaseSucceeded.
//
// MachineOwnerRemediatedCondition on the CAPI Machine is intentionally left
// untouched: it belongs to the Machine's owning controller
// (MachineSet/KCP/MachineDeployment), not to external remediation.
func (s *Service) markRemediationSucceeded(ctx context.Context, msg string) error {
	machinePatchHelper, err := patch.NewHelper(s.scope.Machine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper for machine: %w", err)
	}

	delete(s.scope.Machine.Annotations, clusterv1.RemediateMachineAnnotation)

	if err := machinePatchHelper.Patch(ctx, s.scope.Machine); err != nil {
		return fmt.Errorf("failed to patch machine: %w", err)
	}

	hcloudMachinePatchHelper, err := patch.NewHelper(s.scope.HCloudMachine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper for hcloud machine: %w", err)
	}

	now := metav1.Now()
	s.scope.HCloudMachine.Status.LastRemediatedAt = &now

	if err := hcloudMachinePatchHelper.Patch(ctx, s.scope.HCloudMachine); err != nil {
		return fmt.Errorf("failed to patch hcloud machine: %w", err)
	}

	record.Event(s.scope.HCloudRemediation, "RemediationSucceeded", msg)

	s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseSucceeded
	return nil
}

// markRemediationSkipped escalates to deletion via MachineOwnerRemediated=False
// using RemediationCooldownTriggeredReason, so operators can distinguish a
// cooldown skip from a genuine failure.
func (s *Service) markRemediationSkipped(ctx context.Context, msg string) error {
	patchHelper, err := patch.NewHelper(s.scope.Machine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %w", err)
	}

	// Dual-write: deprecated v1beta1 list AND v1beta2 list. CAPI v1.13 MachineSet
	// reads from the v1beta2 list to trigger Machine deletion.
	deprecatedv1beta1conditions.MarkFalse(
		s.scope.Machine,
		clusterv1.MachineOwnerRemediatedV1Beta1Condition,
		infrav1.RemediationCooldownTriggeredReason,
		clusterv1.ConditionSeverityWarning,
		"Remediation cooldown active (machine will be deleted): %s", msg,
	)
	conditions.Set(s.scope.Machine, metav1.Condition{
		Type:    clusterv1.MachineOwnerRemediatedCondition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.RemediationCooldownTriggeredReason,
		Message: fmt.Sprintf("Remediation cooldown active (machine will be deleted): %s", msg),
	})

	if err := patchHelper.Patch(ctx, s.scope.Machine); err != nil {
		return fmt.Errorf("failed to patch: %w", err)
	}

	record.Event(s.scope.HCloudRemediation, "RemediationSkipped", msg)

	s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseDeleting
	return nil
}

// timeUntilNextRemediation checks if it is time to execute a next remediation step
// and returns seconds to next remediation time.
func (s *Service) timeUntilNextRemediation(now time.Time) time.Duration {
	timeout := s.scope.HCloudRemediation.Spec.Strategy.Timeout.Duration
	// status is not updated yet
	if s.scope.HCloudRemediation.Status.LastRemediated == nil {
		return timeout
	}

	if s.scope.HCloudRemediation.Status.LastRemediated.Add(timeout).Before(now) {
		return time.Duration(0)
	}

	lastRemediated := now.Sub(s.scope.HCloudRemediation.Status.LastRemediated.Time)
	nextRemediation := timeout - lastRemediated + time.Second
	return nextRemediation
}
