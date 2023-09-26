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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
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
func (s *Service) Reconcile(ctx context.Context) (res reconcile.Result, err error) {
	host, err := s.getHost(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = fmt.Errorf("HetznerBareMetalHost not found")
			if err := s.setOwnerRemediatedConditionNew(ctx); err != nil {
				err = fmt.Errorf("failed to set remediated condition on capi machine: %w", err)
				record.Warn(s.scope.BareMetalRemediation, "FailedSettingConditionOnMachine", err.Error())
				return res, err
			}
			return res, nil
		}
		err := fmt.Errorf("failed to find the unhealthy host: %w", err)
		record.Warn(s.scope.BareMetalRemediation, "FailedToFindHost", err.Error())
		return res, err
	}

	// if host is not provisioned, then we do not try to reboot server
	if host.Spec.Status.ProvisioningState != infrav1.StateProvisioned {
		s.scope.Info("Deleting host without remediation", "provisioningState", host.Spec.Status.ProvisioningState)

		if err := s.setOwnerRemediatedConditionNew(ctx); err != nil {
			err := fmt.Errorf("failed to set remediated condition on capi machine: %w", err)
			record.Warn(s.scope.BareMetalRemediation, "FailedSettingConditionOnMachine", err.Error())
			return res, err
		}
		return res, nil
	}

	if s.scope.BareMetalRemediation.Spec.Strategy.Type != infrav1.RemediationTypeReboot {
		record.Warn(s.scope.BareMetalRemediation, "UnsupportedRemediationStrategy", "unsupported remediation strategy")
		return res, nil
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
	}

	return res, nil
}

func (s *Service) handlePhaseRunning(ctx context.Context, host infrav1.HetznerBareMetalHost) (res reconcile.Result, err error) {
	// if host has not been remediated yet, do that now
	if s.scope.BareMetalRemediation.Status.LastRemediated == nil {
		if err := s.remediate(ctx, host); err != nil {
			return res, fmt.Errorf("failed remediate host: %w", err)
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
		return res, fmt.Errorf("failed remediate host: %w", err)
	}

	return res, nil
}

func (s *Service) remediate(ctx context.Context, host infrav1.HetznerBareMetalHost) error {
	var err error

	patchHelper, err := patch.NewHelper(&host, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", host.Kind, host.Namespace, host.Name, err)
	}

	// check if host is not in maintenance mode
	if !*host.Spec.MaintenanceMode {
		// add annotation to host so that it reboots
		host.Annotations, err = addRebootAnnotation(host.Annotations)
		if err != nil {
			return fmt.Errorf("failed to add reboot annotation: %w", err)
		}
	}

	if err := patchHelper.Patch(ctx, &host); err != nil {
		return fmt.Errorf("failed to patch: %s %s/%s %w", host.Kind, host.Namespace, host.Name, err)
	}

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

	// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
	// moves control to CAPI machine controller. The owning controller will do
	// preflight checks and handles the Machine deletion
	s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting

	if err := s.setOwnerRemediatedConditionNew(ctx); err != nil {
		err := fmt.Errorf("failed to set remediated condition on capi machine: %w", err)
		record.Warn(s.scope.BareMetalRemediation, "FailedSettingConditionOnMachine", err.Error())
		return res, err
	}

	return res, nil
}

func (s *Service) getHost(ctx context.Context) (host infrav1.HetznerBareMetalHost, err error) {
	key, err := objectKeyFromAnnotations(s.scope.BareMetalMachine.ObjectMeta.GetAnnotations())
	if err != nil {
		return host, fmt.Errorf("failed to get object key from annotations of bm machine %q: %w", s.scope.BareMetalMachine.Name, err)
	}

	if err := s.scope.Client.Get(ctx, key, &host); err != nil {
		return host, fmt.Errorf("failed to get host %+v: %w", key, err)
	}

	return host, nil
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

// setOwnerRemediatedConditionNew sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck.
func (s *Service) setOwnerRemediatedConditionNew(ctx context.Context) error {
	capiMachine, err := util.GetOwnerMachine(ctx, s.scope.Client, s.scope.BareMetalRemediation.ObjectMeta)
	if err != nil {
		return fmt.Errorf("failed to get capi machine: %w", err)
	}

	patchHelper, err := patch.NewHelper(capiMachine, s.scope.Client)
	if err != nil {
		return fmt.Errorf("failed to init patch helper: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}

	conditions.MarkFalse(
		capiMachine,
		capi.MachineOwnerRemediatedCondition,
		capi.WaitingForRemediationReason,
		capi.ConditionSeverityWarning,
		"remediation through reboot failed",
	)

	if err := patchHelper.Patch(ctx, capiMachine); err != nil {
		return fmt.Errorf("failed to patch: %s %s/%s %w", capiMachine.Kind, capiMachine.Namespace, capiMachine.Name, err)
	}
	return nil
}

func objectKeyFromAnnotations(annotations map[string]string) (client.ObjectKey, error) {
	if annotations == nil {
		return client.ObjectKey{}, fmt.Errorf("unable to get annotations")
	}
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		return client.ObjectKey{}, fmt.Errorf("unable to get HostAnnotation")
	}

	hostNamespace, hostName, err := splitHostKey(hostKey)
	if err != nil {
		return client.ObjectKey{}, fmt.Errorf("failed to parse host key %q: %w", hostKey, err)
	}

	return client.ObjectKey{Name: hostName, Namespace: hostNamespace}, nil
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

func splitHostKey(key string) (namespace, name string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected host key")
	}
	return parts[0], parts[1], nil
}
