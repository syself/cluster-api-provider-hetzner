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

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Service defines struct with machine scope to reconcile Hetzner bare metal remediation.
type Service struct {
	scope *scope.BareMetalRemediationScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalRemediationScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of Hetzner bare metal remediation.
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal remediation", "name", s.scope.BareMetalRemediation.Name)

	// If host is gone, exit early
	host, helper, err := s.getUnhealthyHost(ctx)
	if err != nil {
		s.scope.Error(err, "unable to find a host for unhealthy machine")
		return &ctrl.Result{}, errors.Wrapf(err, "unable to find a host for unhealthy machine")
	}

	// If host is not in state provisioned, then remediate immediately
	if host.Spec.Status.ProvisioningState != infrav1.StateProvisioned {
		log.Info("Deleting host without remediation", "provisioningState", host.Spec.Status.ProvisioningState)
		if err := s.setOwnerRemediatedConditionNew(ctx); err != nil {
			s.scope.Error(err, "error setting cluster api conditions")
			return &ctrl.Result{}, errors.Wrapf(err, "error setting cluster api conditions")
		}
	}

	remediationType := s.scope.BareMetalRemediation.Spec.Strategy.Type

	if remediationType != infrav1.RebootRemediationStrategy {
		s.scope.Info("unsupported remediation strategy")
		return &ctrl.Result{}, nil
	}

	if remediationType == infrav1.RebootRemediationStrategy {
		// If no phase set, default to running
		if s.scope.BareMetalRemediation.Status.Phase == "" {
			s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseRunning
		}

		switch s.scope.BareMetalRemediation.Status.Phase {
		case infrav1.PhaseRunning:
			// host is not rebooted yet
			if s.scope.BareMetalRemediation.Status.LastRemediated == nil {
				s.scope.Info("Rebooting the host")
				err := s.setRebootAnnotation(ctx, host, helper)
				if err != nil {
					s.scope.Error(err, "error setting reboot annotation")
					return &ctrl.Result{}, errors.Wrap(err, "error setting reboot annotation")
				}
				now := metav1.Now()
				s.scope.BareMetalRemediation.Status.LastRemediated = &now
				s.scope.BareMetalRemediation.Status.RetryCount++
			}

			if s.scope.BareMetalRemediation.Spec.Strategy.RetryLimit > 0 &&
				s.scope.BareMetalRemediation.Spec.Strategy.RetryLimit > s.scope.BareMetalRemediation.Status.RetryCount {
				okToRemediate, nextRemediation := s.timeToRemediate(s.scope.BareMetalRemediation.Spec.Strategy.Timeout.Duration)

				if okToRemediate {
					err := s.setRebootAnnotation(ctx, host, helper)
					if err != nil {
						s.scope.Error(err, "error setting reboot annotation")
						return &ctrl.Result{}, errors.Wrapf(err, "error setting reboot annotation")
					}
					now := metav1.Now()
					s.scope.BareMetalRemediation.Status.LastRemediated = &now
					s.scope.BareMetalRemediation.Status.RetryCount++
				}

				if nextRemediation > 0 {
					// Not yet time to remediate, requeue
					return &ctrl.Result{RequeueAfter: nextRemediation}, nil
				}
			} else {
				s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseWaiting
			}
		case infrav1.PhaseWaiting:
			okToStop, nextCheck := s.timeToRemediate(s.scope.BareMetalRemediation.Spec.Strategy.Timeout.Duration)

			if okToStop {
				s.scope.BareMetalRemediation.Status.Phase = infrav1.PhaseDeleting
				// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
				// moves control to CAPI machine controller. The owning controller will do
				// preflight checks and handles the Machine deletion

				err = s.setOwnerRemediatedConditionNew(ctx)
				if err != nil {
					s.scope.Error(err, "error setting cluster api conditions")
					return &ctrl.Result{}, errors.Wrapf(err, "error setting cluster api conditions")
				}
			}

			if nextCheck > 0 {
				// Not yet time to stop remediation, requeue
				return &ctrl.Result{RequeueAfter: nextCheck}, nil
			}
		default:
		}
	}
	return &ctrl.Result{}, nil
}

func (s *Service) getUnhealthyHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, *patch.Helper, error) {
	host, err := s.getHost(ctx)
	if err != nil || host == nil {
		return host, nil, err
	}
	helper, err := patch.NewHelper(host, s.scope.Client)
	return host, helper, err
}

func (s *Service) getHost(ctx context.Context) (*infrav1.HetznerBareMetalHost, error) {
	annotations := s.scope.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		err := fmt.Errorf("unable to get %s annotations", s.scope.BareMetalMachine.Name)
		return nil, err
	}
	hostKey, ok := annotations[infrav1.HostAnnotation]
	if !ok {
		err := fmt.Errorf("unable to get %s HostAnnotation", s.scope.BareMetalMachine.Name)
		return nil, err
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		s.scope.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, err
	}

	host := infrav1.HetznerBareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = s.scope.Client.Get(ctx, key, &host)
	if apierrors.IsNotFound(err) {
		s.scope.Info("Annotated host not found", "host", hostKey)
		return nil, err
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// setRebootAnnotation sets reboot annotation on unhealthy host.
func (s *Service) setRebootAnnotation(ctx context.Context, host *infrav1.HetznerBareMetalHost, helper *patch.Helper) error {
	s.scope.Info("Adding Reboot annotation to host", "host", host.Name)
	reboot := infrav1.RebootAnnotationArguments{}
	reboot.Type = infrav1.RebootTypeHardware
	marshalledMode, err := json.Marshal(reboot)

	if err != nil {
		return err
	}

	host.Annotations[infrav1.RebootAnnotation] = string(marshalledMode)
	return helper.Patch(ctx, host)
}

// timeToRemediate checks if it is time to execute a next remediation step
// and returns seconds to next remediation time.
func (s *Service) timeToRemediate(timeout time.Duration) (bool, time.Duration) {
	now := time.Now()

	// status is not updated yet
	if s.scope.BareMetalRemediation.Status.LastRemediated == nil {
		return false, timeout
	}

	if s.scope.BareMetalRemediation.Status.LastRemediated.Add(timeout).Before(now) {
		return true, time.Duration(0)
	}

	lastRemediated := now.Sub(s.scope.BareMetalRemediation.Status.LastRemediated.Time)
	nextRemediation := timeout - lastRemediated + time.Second
	return false, nextRemediation
}

// setOwnerRemediatedConditionNew sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck.
func (s *Service) setOwnerRemediatedConditionNew(ctx context.Context) error {
	capiMachine, err := s.getCapiMachine(ctx)
	if err != nil {
		s.scope.Info("Unable to fetch CAPI Machine")
		return err
	}

	machineHelper, err := patch.NewHelper(capiMachine, s.scope.Client)
	if err != nil {
		s.scope.Info("Unable to create patch helper for Machine")
		return err
	}
	conditions.MarkFalse(capiMachine, capi.MachineOwnerRemediatedCondition, capi.WaitingForRemediationReason, capi.ConditionSeverityWarning, "")
	err = machineHelper.Patch(ctx, capiMachine)
	if err != nil {
		s.scope.Info("Unable to patch Machine", "machine", capiMachine)
		return err
	}
	return nil
}

// getCapiMachine returns CAPI machine object owning the current resource.
func (s *Service) getCapiMachine(ctx context.Context) (*capi.Machine, error) {
	capiMachine, err := util.GetOwnerMachine(ctx, s.scope.Client, s.scope.BareMetalRemediation.ObjectMeta)
	if err != nil {
		s.scope.Error(err, "metal3Remediation's owner Machine could not be retrieved")
		return nil, errors.Wrapf(err, "metal3Remediation's owner Machine could not be retrieved")
	}
	return capiMachine, nil
}
