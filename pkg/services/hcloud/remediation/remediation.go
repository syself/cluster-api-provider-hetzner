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

	"github.com/hetznercloud/hcloud-go/hcloud"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
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
func (s *Service) Reconcile(ctx context.Context) (res ctrl.Result, err error) {
	s.scope.Info("Reconciling hcloud remediation", "name", s.scope.HCloudRemediation.Name)

	server, err := s.findServer(ctx)
	if err != nil {
		return res, fmt.Errorf("failed to find the server of unhealthy machine: %w", err)
	}

	remediationType := s.scope.HCloudRemediation.Spec.Strategy.Type

	if remediationType != infrav1.RemediationTypeReboot {
		s.scope.Info("unsupported remediation strategy")
		record.Warnf(s.scope.HCloudRemediation, "UnsupportedRemdiationStrategy", "remediation strategy %q is unsupported", remediationType)
		return res, nil
	}

	if remediationType == infrav1.RemediationTypeReboot {
		// If no phase set, default to running
		if s.scope.HCloudRemediation.Status.Phase == "" {
			s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseRunning
		}

		switch s.scope.HCloudRemediation.Status.Phase {
		case infrav1.PhaseRunning:
			return s.handlePhaseRunning(ctx, server)
		case infrav1.PhaseWaiting:
			return s.handlePhaseWaiting(ctx)
		default:
		}
	}
	return res, nil
}

func (s *Service) handlePhaseRunning(ctx context.Context, server *hcloud.Server) (res ctrl.Result, err error) {
	// server is not rebooted yet
	if s.scope.HCloudRemediation.Status.LastRemediated == nil {
		s.scope.Info("Rebooting the server")

		if err := s.rebootServer(ctx, server); err != nil {
			return res, fmt.Errorf("failed to reboot server: %w", err)
		}

		now := metav1.Now()
		s.scope.HCloudRemediation.Status.LastRemediated = &now
		s.scope.HCloudRemediation.Status.RetryCount++
	}

	retryLimit := s.scope.HCloudRemediation.Spec.Strategy.RetryLimit
	retryCount := s.scope.HCloudRemediation.Status.RetryCount

	if retryLimit > 0 && retryLimit > retryCount {
		okToRemediate, nextRemediation := s.timeToRemediate()

		if okToRemediate {
			s.scope.Info("Rebooting the server")

			if err := s.rebootServer(ctx, server); err != nil {
				return res, fmt.Errorf("failed to reboot server: %w", err)
			}

			now := metav1.Now()
			s.scope.HCloudRemediation.Status.LastRemediated = &now
			s.scope.HCloudRemediation.Status.RetryCount++
		}

		if nextRemediation > 0 {
			// Not yet time to remediate, requeue
			return ctrl.Result{RequeueAfter: nextRemediation}, nil
		}
	} else {
		s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseWaiting
	}
	return res, nil
}

func (s *Service) handlePhaseWaiting(ctx context.Context) (res ctrl.Result, err error) {
	okToStop, nextCheck := s.timeToRemediate()

	if okToStop {
		s.scope.HCloudRemediation.Status.Phase = infrav1.PhaseDeleting
		// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
		// moves control to CAPI machine controller. The owning controller will do
		// preflight checks and handles the Machine deletion

		if err := s.setOwnerRemediatedCondition(ctx); err != nil {
			return res, fmt.Errorf("failed to set conditions on CAPI machine: %w", err)
		}
	}

	if nextCheck > 0 {
		// Not yet time to stop remediation, requeue
		return ctrl.Result{RequeueAfter: nextCheck}, nil
	}
	return res, nil
}

func (s *Service) findServer(ctx context.Context) (*hcloud.Server, error) {
	serverID, err := s.scope.ServerIDFromProviderID()
	if err != nil {
		return nil, fmt.Errorf("failed to get serverID from providerID: %w", err)
	}

	server, err := s.scope.HCloudClient.GetServer(ctx, serverID)
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HCloudRemediation, infrav1.RateLimitExceeded)
			record.Event(s.scope.HCloudRemediation,
				"RateLimitExceeded",
				"exceeded rate limit with calling hcloud function GetServer",
			)
		}
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	return server, nil
}

func (s *Service) rebootServer(ctx context.Context, server *hcloud.Server) error {
	s.scope.Info("Rebooting server", "server", server.ID)
	if _, err := s.scope.HCloudClient.RebootServer(ctx, server); err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HCloudRemediation, infrav1.RateLimitExceeded)
			record.Event(s.scope.HCloudRemediation,
				"RateLimitExceeded",
				"exceeded rate limit with calling hcloud function RebootServer",
			)
		}
		return fmt.Errorf("failed to reboot server %v: %w", server.ID, err)
	}
	return nil
}

// timeToRemediate checks if it is time to execute a next remediation step
// and returns seconds to next remediation time.
func (s *Service) timeToRemediate() (bool, time.Duration) {
	now := time.Now()
	timeout := s.scope.HCloudRemediation.Spec.Strategy.Timeout.Duration
	lastRemediated := s.scope.HCloudRemediation.Status.LastRemediated

	// status is not updated yet
	if lastRemediated == nil {
		return false, timeout
	}

	if lastRemediated.Add(timeout).Before(now) {
		return true, time.Duration(0)
	}

	timeElapsedSinceLastRemediation := now.Sub(lastRemediated.Time)
	nextRemediation := timeout - timeElapsedSinceLastRemediation + time.Second
	return false, nextRemediation
}

// setOwnerRemediatedCondition sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck.
func (s *Service) setOwnerRemediatedCondition(ctx context.Context) error {
	conditions.MarkFalse(
		s.scope.Machine,
		capi.MachineOwnerRemediatedCondition,
		capi.WaitingForRemediationReason,
		capi.ConditionSeverityWarning,
		"",
	)
	if err := s.scope.PatchMachine(ctx); err != nil {
		return fmt.Errorf("failed to patch machine: %w", err)
	}
	return nil
}
