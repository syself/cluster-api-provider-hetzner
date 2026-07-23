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

// Package server implements functions to manage the lifecycle of HCloud servers.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/imageurlcommand"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

const (
	serverOffTimeout = 10 * time.Minute

	// requeueImmediately gets used to requeue "now". One second gets used to make
	// it unlikely that the next Reconcile reads stale data from the local cache.
	requeueImmediately = 1 * time.Second

	actionDone = -1

	preRescueOSImage = "ubuntu-24.04"
)

var hcloudImageURLCommandDir = "/shared"

var errServerCreateNotPossible = errors.New("server create not possible - need action")

var errServerCreateStopReconcile = errors.New("stopped Reconciling")

var errSSHKeyMisconfigured = errors.New("SSH key misconfigured")

// Service defines struct with machine scope to reconcile HCloudMachines.
type Service struct {
	scope *scope.MachineScope
}

// setBootState sets the BootState and logs the transition, together with how long the
// machine was in the previous state.
func (s *Service) setBootState(bootState infrav1.HCloudBootState) {
	hm := s.scope.HCloudMachine
	if hm.Status.BootState == bootState {
		return
	}
	durationOfPreviousState := time.Duration(0)
	if !hm.Status.BootStateSince.IsZero() {
		durationOfPreviousState = time.Since(hm.Status.BootStateSince.Time).Round(time.Millisecond)
	}
	s.scope.Info("BootState changed",
		"from", hm.Status.BootState,
		"to", bootState,
		"durationOfPreviousState", durationOfPreviousState)
	hm.SetBootState(bootState)
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.MachineScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HCloudMachines.
func (s *Service) Reconcile(ctx context.Context) (res reconcile.Result, err error) {
	// delete the deprecated condition from existing machine objects
	v1beta1conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedInstanceReadyCondition)
	v1beta1conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedInstanceBootstrapReadyCondition)
	v1beta1conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedRateLimitExceededCondition)

	if s.scope.HCloudMachine.Status.BootState == infrav1.HCloudBootStateProvisioningFailed {
		// This hcloud machine will be removed soon.
		s.scope.Info("hcloudmachine: ProvisioningFailed. Not reconciling this machine.")
		return reconcile.Result{}, nil
	}

	// detect failure domain
	failureDomain, err := s.scope.GetFailureDomain()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get failure domain: %w", err)
	}

	// set region in status of machine
	s.scope.SetRegion(failureDomain)

	// waiting for bootstrap data to be ready
	if !s.scope.IsBootstrapDataReady() {
		v1beta1conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.BootstrapReadyCondition,
			infrav1.BootstrapNotReadyReason,
			clusterv1beta1.ConditionSeverityInfo,
			"bootstrap not ready yet",
		)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerWaitingForBootstrapDataV1Beta2Reason,
			Message: "bootstrap not ready yet",
		})
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	v1beta1conditions.MarkTrue(s.scope.HCloudMachine, infrav1.BootstrapReadyCondition)

	sinceBootStateChanged := time.Duration(0)
	if !s.scope.HCloudMachine.Status.BootStateSince.IsZero() {
		sinceBootStateChanged = time.Since(s.scope.HCloudMachine.Status.BootStateSince.Time).Round(time.Millisecond)
	}
	bootState := s.scope.HCloudMachine.Status.BootState
	s.scope.V(1).Info("Reconciling BootState",
		"bootState", bootState,
		"sinceBootStateChanged", sinceBootStateChanged)

	switch bootState {
	case infrav1.HCloudBootStateUnset:
		return s.handleBootStateUnset(ctx)
	case infrav1.HCloudBootStateInitializing:
		return s.handleBootStateInitializing(ctx)
	case infrav1.HCloudBootStateEnablingRescue:
		return s.handleBootStateEnablingRescue(ctx)
	case infrav1.HCloudBootStateBootingToRescue:
		return s.handleBootStateBootingToRescue(ctx)
	case infrav1.HCloudBootStateRunningImageCommand:
		return s.handleBootStateRunningImageCommand(ctx)
	case infrav1.HCloudBootStateBootingToRealOS:
		return s.handleBootingToRealOS(ctx)
	case infrav1.HCloudBootStateOperatingSystemRunning:
		return s.handleOperatingSystemRunning(ctx)
	default:
		return reconcile.Result{}, fmt.Errorf("unknown BootState: %s", bootState)
	}
}

// handleBootStateUnset is first state for both ways (imageName/snapshot and imageURL).
func (s *Service) handleBootStateUnset(ctx context.Context) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine

	if hm.Status.BootStateSince.IsZero() {
		hm.Status.BootStateSince = metav1.Now()
	}

	durationOfState := time.Since(hm.Status.BootStateSince.Time)
	if durationOfState > 6*time.Minute {
		// timeout. Something has failed.
		timeoutMsg := fmt.Sprintf("boot state unset timed out, in this state since %s", durationOfState.Round(time.Second).String())

		v1beta1Reason := "HandleBootStateUnsetTimedOut"
		v1beta1Msg := timeoutMsg
		if existing := v1beta1conditions.Get(hm, infrav1.ServerCreateSucceededCondition); existing != nil {
			v1beta1Reason = existing.Reason
			if existing.Message != "" {
				v1beta1Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		v1beta2Reason := infrav1.HCloudMachineBootStateUnsetTimedOutV1Beta2Reason
		v1beta2Msg := timeoutMsg
		if existing := v1beta2conditions.Get(hm, infrav1.HCloudMachineServerCreatedV1Beta2Condition); existing != nil {
			v1beta2Reason = existing.Reason
			if existing.Message != "" {
				v1beta2Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		err := s.scope.SetErrorAndRemediate(ctx, v1beta2Msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		s.scope.Error(nil, v1beta2Msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerCreateSucceededCondition,
			v1beta1Reason, clusterv1beta1.ConditionSeverityWarning,
			"%s", v1beta1Msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  v1beta2Reason,
			Message: v1beta2Msg,
		})
		return reconcile.Result{}, nil
	}

	if hm.Spec.ProviderID != nil && *hm.Spec.ProviderID != "" && hm.Spec.ImageURL == "" {
		// This machine seems to be an existing machine which was created before introducing
		// Status.BootState.

		var msg string
		if !hm.Status.Ready {
			s.setBootState(infrav1.HCloudBootStateBootingToRealOS)
		} else {
			s.setBootState(infrav1.HCloudBootStateOperatingSystemRunning)
		}
		msg = fmt.Sprintf("Updating old resource (pre BootState) %s", hm.Status.BootState)

		s.scope.Info(msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"HandleBootStateUnset", clusterv1beta1.ConditionSeverityInfo,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineBootStateInitializingV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil
	}

	// The imageURL flow installs the image via SSH in the rescue system, so it needs a valid
	// SSH private key. Check that before creating the server, so that no server gets created
	// when the key is misconfigured. Other failures could also mean a network failure while
	// trying to access the api-server, so they get retried.
	if hm.Spec.ImageURL != "" {
		_, err := s.getSSHPrivateKey(ctx)
		if err != nil {
			s.scope.Error(err, "")
			if errors.Is(err, errSSHKeyMisconfigured) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
		}
		v1beta1conditions.MarkTrue(s.scope.HCloudMachine, infrav1.SSHPrivateKeyAvailableCondition)
	}

	server, image, err := s.createServerFromImageNameOrURL(ctx)
	if err != nil {
		// If it is an unauthorized error i.e. wrong HCloudToken do not return an error.
		// As there is no point retrying with invalid credentials.
		if errors.Is(err, hcloudclient.ErrUnauthorized) {
			v1beta1conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.HCloudTokenAvailableCondition,
				infrav1.HCloudCredentialsInvalidReason,
				clusterv1beta1.ConditionSeverityError,
				"wrong hcloud token",
			)
			v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: "wrong hcloud token",
			})

			return reconcile.Result{}, nil
		}

		// createServer hit a uniqueness_error that adoption (taking over an existing server) could
		// not resolve. Unlike invalid_input/resource_unavailable, this can clear from the outside:
		// the conflicting server may be deleted, or this Machine may be replaced with a new name.
		// Nothing watches HCloud, so we requeue on a slow interval to retry, otherwise such a change
		// would never trigger a new attempt. createServer already set ServerCreateSucceededCondition
		// to false, so we only requeue here.
		if hcloud.IsError(err, hcloud.ErrorCodeUniquenessError) {
			return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
		}

		// Terminal errors like invalid_input (e.g. unsupported location for server type)
		// or resource_unavailable (e.g. server location disabled) will never succeed on retry.
		// Mark the machine as irrecoverably failed and stop reconciling.
		if hcloud.IsError(err, hcloud.ErrorCodeInvalidInput) || hcloud.IsError(err, hcloud.ErrorCodeResourceUnavailable) {
			msg := fmt.Sprintf(
				"Server creation failed with an irrecoverable error: %s. If the requested resources (server type or location) become available again, delete the Machine to trigger a new creation attempt.",
				err.Error(),
			)
			v1beta1conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.ServerCreateSucceededCondition,
				infrav1.ServerCreateFailedIrrecoverableErrorReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
				Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineServerCreationFailedIrrecoverablyV1Beta2Reason,
				Message: msg,
			})
			return reconcile.Result{}, nil
		}
		if errors.Is(err, errServerCreateNotPossible) {
			err = fmt.Errorf("createServerFromImageNameOrURL failed: %w", err)
			s.scope.Error(err, "")
			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		if errors.Is(err, errServerCreateStopReconcile) {
			err = fmt.Errorf("createServerFromImageNameOrURL failed: %w", err)
			s.scope.Error(err, "")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, fmt.Errorf("failed to create server: %w", err)
	}

	v1beta1conditions.MarkTrue(s.scope.HCloudMachine, infrav1.HCloudTokenAvailableCondition)
	v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
		Type:   infrav1.HCloudTokenAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudTokenAvailableV1Beta2Reason,
	})

	updateHCloudMachineStatusFromServer(hm, server)

	s.scope.SetProviderID(server.ID)

	// If server creation was successful, but reconciliation failed afterward, its
	// condition might not be true yet.
	v1beta1conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:   infrav1.HCloudMachineServerCreatedV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineServerCreatedV1Beta2Reason,
	})

	// These values get only used **once** after the server got created.
	var requeueAfter time.Duration
	if hm.Spec.ImageName != "" {
		// The imageName flow boots the real image directly.
		// Provisioning from a hcloud image like ubuntu-YY.MM takes roughly 11 seconds.
		// Provisioning from a snapshot takes roughly 140 seconds.
		// We do not want to do too many api-calls (rate-limiting). So we differentiate
		// between both cases.
		requeueAfter = 140 * time.Second
		if image.RapidDeploy {
			requeueAfter = 10 * time.Second
		}
	} else {
		// The imageURL flow created the server powered off. Only the server create action needs
		// to finish before the rescue system can be enabled.
		requeueAfter = 15 * time.Second
	}
	v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
		"ProvisioningServer", clusterv1beta1.ConditionSeverityInfo,
		"Provisioning and rebooting server")
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HCloudMachineProvisioningServerV1Beta2Reason,
		Message: "Provisioning and rebooting server",
	})
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

// handleBootStateInitializing is for provisioning with imageURL and image-url-command.
func (s *Service) handleBootStateInitializing(ctx context.Context) (res reconcile.Result, reterr error) {
	hm := s.scope.HCloudMachine

	durationOfState := time.Since(hm.Status.BootStateSince.Time)
	if durationOfState > 6*time.Minute {
		// timeout. Something has failed.
		timeoutMsg := fmt.Sprintf("boot state initializing timed out, in this state since %s", durationOfState.Round(time.Second).String())

		v1beta1Reason := "BootStateInitializingTimedOut"
		v1beta1Msg := timeoutMsg
		if existing := v1beta1conditions.Get(hm, infrav1.ServerProvisionedCondition); existing != nil {
			v1beta1Reason = existing.Reason
			if existing.Message != "" {
				v1beta1Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		v1beta2Reason := infrav1.HCloudMachineBootStateInitializingTimedOutV1Beta2Reason
		v1beta2Msg := timeoutMsg
		if existing := v1beta2conditions.Get(hm, infrav1.HCloudMachineServerProvisionedV1Beta2Condition); existing != nil {
			v1beta2Reason = existing.Reason
			if existing.Message != "" {
				v1beta2Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		err := s.scope.SetErrorAndRemediate(ctx, v1beta2Msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		s.scope.Error(nil, v1beta2Msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			v1beta1Reason, clusterv1beta1.ConditionSeverityWarning,
			"%s", v1beta1Msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  v1beta2Reason,
			Message: v1beta2Msg,
		})
		return reconcile.Result{}, nil
	}

	// ActionIDCreateServer gets stored by createServerFromImageURL before the boot state becomes
	// Initializing. This guard catches it early if that behavior ever changes.
	if hm.Status.ExternalIDs.ActionIDCreateServer == 0 {
		msg := "ActionIDCreateServer is missing in the status.externalIDs, cannot check whether the server is provisioned. Machine will be remediated"
		s.scope.Error(nil, msg)
		err := s.scope.SetErrorAndRemediate(ctx, msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"ActionIDCreateServerNotSet", clusterv1beta1.ConditionSeverityWarning, "%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineActionIDCreateServerNotSetV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{}, nil
	}

	// The server was created with StartAfterCreate=false and is still powered off. Wait until
	// the server create action is finished, which means the server is provisioned and unlocked. After
	// that the rescue system can be enabled. Once finished, ActionIDCreateServer is set to actionDone
	// so a later EnableRescueSystem retry does not fetch the finished action again.
	if hm.Status.ExternalIDs.ActionIDCreateServer != actionDone {
		action, err := s.scope.HCloudClient.GetAction(ctx, hm.Status.ExternalIDs.ActionIDCreateServer)
		if err != nil {
			if handleUnauthorized(hm, err) {
				return reconcile.Result{}, nil
			}
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return reconcile.Result{}, handleRateLimit(hm, err, "GetAction", "failed to get server create action")
			}

			// If this error persists, then the BootState will time out, and a new
			// machine will be created.
			err = fmt.Errorf("GetAction failed: %w", err)
			s.scope.Error(err, "")
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"GettingServerCreationStatusFailed", clusterv1beta1.ConditionSeverityWarning,
				"%s", err.Error())
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionUnknown,
				Reason:  infrav1.HCloudMachineGettingServerCreationStatusFailedV1Beta2Reason,
				Message: err.Error(),
			})
			return reconcile.Result{}, err
		}
		markHCloudTokenAvailable(hm)

		if action.Finished.IsZero() {
			// not finished yet.
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"CreatingServer", clusterv1beta1.ConditionSeverityInfo,
				"Waiting until the server is created")
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineCreatingServerV1Beta2Reason,
				Message: "Waiting until the server is created",
			})
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		err = action.Error()
		if err != nil {
			err = fmt.Errorf("action %+v failed (wait for server create): %w", action, err)
			msg := err.Error()
			s.scope.Error(err, "")
			remediateErr := s.scope.SetErrorAndRemediate(ctx, msg)
			if remediateErr != nil {
				return reconcile.Result{}, remediateErr
			}
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"CreationFailed", clusterv1beta1.ConditionSeverityWarning,
				"%s", msg)
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineServerCreationFailedV1Beta2Reason,
				Message: msg,
			})
			return reconcile.Result{}, nil
		}

		// The server create action finished successfully.
		hm.Status.ExternalIDs.ActionIDCreateServer = actionDone
	}

	// The server create action is finished. The server is provisioned and still powered off.

	_, hcloudSSHKeys, err := s.getSSHKeys(ctx)
	if err != nil {
		return res, fmt.Errorf("getSSHKeys failed: %w", err)
	}

	rescueOpts := &hcloud.ServerEnableRescueOpts{
		Type:    hcloud.ServerRescueTypeLinux64,
		SSHKeys: hcloudSSHKeys,
	}

	serverID, err := s.scope.ServerIDFromProviderID()
	if err != nil {
		return res, fmt.Errorf("ServerIDFromProviderID failed: %w", err)
	}

	result, err := s.scope.HCloudClient.EnableRescueSystem(ctx, &hcloud.Server{ID: serverID}, rescueOpts)
	if err != nil {
		if handleUnauthorized(hm, err) {
			return reconcile.Result{}, nil
		}
		if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
			// a fresh server is locked only for a short time after create, so a short retry
			// interval is enough
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"EnablingRescueSystemFailed", clusterv1beta1.ConditionSeverityInfo,
				"EnableRescueSystem: server locked. Will retry")
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineEnablingRescueSystemFailedV1Beta2Reason,
				Message: "EnableRescueSystem: server locked. Will retry",
			})
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return res, handleRateLimit(hm, err, "EnableRescueSystem", "failed to enable rescue system")
	}
	markHCloudTokenAvailable(hm)

	// The API of hetzner is async. We get an Action-ID as result. We need to wait until the action
	// is done. After that we can power the server on, so that it boots into the rescue system.
	hm.Status.ExternalIDs.ActionIDEnableRescueSystem = result.Action.ID

	s.setBootState(infrav1.HCloudBootStateEnablingRescue)

	v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
		"WaitForRescueSystem", clusterv1beta1.ConditionSeverityInfo,
		"waiting for rescue system to be enabled")
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HCloudMachineWaitingForRescueSystemV1Beta2Reason,
		Message: "waiting for rescue system to be enabled",
	})
	return reconcile.Result{RequeueAfter: requeueImmediately}, nil
}

// handleBootStateEnablingRescue is for provisioning with imageURL and image-url-command.
func (s *Service) handleBootStateEnablingRescue(ctx context.Context) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine

	durationOfState := time.Since(hm.Status.BootStateSince.Time)
	if durationOfState > 6*time.Minute {
		// timeout. Something has failed.
		timeoutMsg := fmt.Sprintf("enabling rescue system timed out, in this state since %s", durationOfState.Round(time.Second).String())

		v1beta1Reason := "EnablingRescueTimedOut"
		v1beta1Msg := timeoutMsg
		if existing := v1beta1conditions.Get(hm, infrav1.ServerProvisionedCondition); existing != nil {
			v1beta1Reason = existing.Reason
			if existing.Message != "" {
				v1beta1Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		v1beta2Reason := infrav1.HCloudMachineEnablingRescueTimedOutV1Beta2Reason
		v1beta2Msg := timeoutMsg
		if existing := v1beta2conditions.Get(hm, infrav1.HCloudMachineServerProvisionedV1Beta2Condition); existing != nil {
			v1beta2Reason = existing.Reason
			if existing.Message != "" {
				v1beta2Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		s.scope.Error(nil, v1beta2Msg)
		err := s.scope.SetErrorAndRemediate(ctx, v1beta2Msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			v1beta1Reason, clusterv1beta1.ConditionSeverityWarning, "%s", v1beta1Msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  v1beta2Reason,
			Message: v1beta2Msg,
		})
		return reconcile.Result{}, nil
	}

	// ActionIDEnableRescueSystem gets stored by handleBootStateInitializing before the boot
	// state becomes EnablingRescue. This guard catches it early if that behavior ever changes.
	if hm.Status.ExternalIDs.ActionIDEnableRescueSystem == 0 {
		msg := "ActionIDEnableRescueSystem is missing in the status.externalIDs, cannot check whether the rescue system is enabled. Machine will be remediated"
		s.scope.Error(nil, msg)
		err := s.scope.SetErrorAndRemediate(ctx, msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"ActionIDForEnablingRescueSystemNotSet", clusterv1beta1.ConditionSeverityWarning, "%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineActionIDForEnablingRescueSystemNotSetV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{}, nil
	}

	if hm.Status.ExternalIDs.ActionIDEnableRescueSystem != actionDone {
		action, err := s.scope.HCloudClient.GetAction(ctx, hm.Status.ExternalIDs.ActionIDEnableRescueSystem)
		if err != nil {
			if handleUnauthorized(hm, err) {
				return reconcile.Result{}, nil
			}
			if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
				return reconcile.Result{}, handleRateLimit(hm, err, "GetAction", "failed to get enabling rescue action")
			}

			// If this error persists, then the BootState will time out, and a new
			// machine will be created.
			err = fmt.Errorf("GetAction failed: %w", err)
			s.scope.Error(err, "")
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"EnablingRescueGetActionFailed", clusterv1beta1.ConditionSeverityWarning,
				"%s", err.Error())
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionUnknown,
				Reason:  infrav1.HCloudMachineEnablingRescueGetActionFailedV1Beta2Reason,
				Message: err.Error(),
			})
			return reconcile.Result{}, err
		}
		markHCloudTokenAvailable(hm)

		if action.Finished.IsZero() {
			// not finished yet.
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"WaitingForEnablingRescueAction", clusterv1beta1.ConditionSeverityInfo,
				"Waiting until Action RescueEnabled is finished")
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineWaitingForEnablingRescueActionV1Beta2Reason,
				Message: "Waiting until Action RescueEnabled is finished",
			})
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		err = action.Error()
		if err != nil {
			err = fmt.Errorf("action %+v failed (wait for rescue enabled): %w", action, err)
			msg := err.Error()
			s.scope.Error(err, "")
			remediateErr := s.scope.SetErrorAndRemediate(ctx, msg)
			if remediateErr != nil {
				return reconcile.Result{}, remediateErr
			}
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"EnablingRescueActionFailed", clusterv1beta1.ConditionSeverityWarning,
				"%s", msg)
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineEnablingRescueActionFailedV1Beta2Reason,
				Message: msg,
			})
			return reconcile.Result{}, nil
		}

		s.scope.Info("Action RescueEnabled is finished",
			"actionDuration", action.Finished.Sub(action.Started),
			"finishedSince", time.Since(action.Finished),
			"actionStatus", action.Status)

		hm.Status.ExternalIDs.ActionIDEnableRescueSystem = actionDone
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"EnablingRescueActionDone", clusterv1beta1.ConditionSeverityInfo,
			"Action RescueEnabled is finished")
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineEnablingRescueActionDoneV1Beta2Reason,
			Message: "Action RescueEnabled is finished",
		})
		// Requeue immediately as Hetzner accepts the power on directly after the enable rescue action is finished.
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil
	}

	serverID, err := s.scope.ServerIDFromProviderID()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("ServerIDFromProviderID failed: %w", err)
	}
	// The server was created with StartAfterCreate=false and has never been started, so
	// powering it on boots it directly into the rescue system.
	if err := s.scope.HCloudClient.PowerOnServer(ctx, &hcloud.Server{ID: serverID}); err != nil {
		if handleUnauthorized(hm, err) {
			return reconcile.Result{}, nil
		}
		if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
			// a fresh server is locked only for a short time after create, so a short retry
			// interval is enough
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"PowerOnServerFailed", clusterv1beta1.ConditionSeverityInfo,
				"PowerOnServer: server locked. Will retry")
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachinePoweringOnServerFailedV1Beta2Reason,
				Message: "PowerOnServer: server locked. Will retry",
			})
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return reconcile.Result{}, handleRateLimit(hm, err, "PowerOnServer", "failed to power on server")
	}
	markHCloudTokenAvailable(hm)

	s.setBootState(infrav1.HCloudBootStateBootingToRescue)
	v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
		"BootingToRescue", clusterv1beta1.ConditionSeverityInfo,
		"power on to rescue started")
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HCloudMachineBootingToRescueV1Beta2Reason,
		Message: "power on to rescue started",
	})
	// The next state (BootingToRescue) polls via SSH, which costs no hcloud API calls, but
	// powering on is not instant, so wait a bit before the first attempt instead of retrying
	// immediately.
	return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
}

// handleBootStateBootingToRescue is for provisioning with imageURL and image-url-command.
func (s *Service) handleBootStateBootingToRescue(ctx context.Context) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine

	durationOfState := time.Since(hm.Status.BootStateSince.Time)
	if durationOfState > 6*time.Minute {
		// timeout. Something has failed.
		timeoutMsg := fmt.Sprintf("reaching rescue system timed out, in this state since %s", durationOfState.Round(time.Second).String())

		v1beta1Reason := "BootingToRescueTimedOut"
		v1beta1Msg := timeoutMsg
		if existing := v1beta1conditions.Get(hm, infrav1.ServerProvisionedCondition); existing != nil {
			v1beta1Reason = existing.Reason
			if existing.Message != "" {
				v1beta1Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		v1beta2Reason := infrav1.HCloudMachineBootingToRescueTimedOutV1Beta2Reason
		v1beta2Msg := timeoutMsg
		if existing := v1beta2conditions.Get(hm, infrav1.HCloudMachineServerProvisionedV1Beta2Condition); existing != nil {
			v1beta2Reason = existing.Reason
			if existing.Message != "" {
				v1beta2Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		err := s.scope.SetErrorAndRemediate(ctx, v1beta2Msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		s.scope.Error(nil, v1beta2Msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			v1beta1Reason, clusterv1beta1.ConditionSeverityWarning,
			"%s", v1beta1Msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  v1beta2Reason,
			Message: v1beta2Msg,
		})
		return reconcile.Result{}, nil
	}

	// The server is freshly created and was never started before this boot cycle, so there is no
	// prior OS it could mistakenly reach over SSH. Attempt SSH directly instead of first checking
	// server.RescueEnabled via a live GetServer call - ECONNREFUSED below already covers "server
	// has not yet rebooted into rescue system".
	sshClient, err := s.getSSHClient(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getSSHClient failed (waiting for rescue running): %w", err)
	}

	output := sshClient.GetHostName(ctx)
	err = output.Err
	if err != nil {
		var msg string
		if errors.Is(err, syscall.ECONNREFUSED) {
			// This is common. Provide a nice message.
			msg = "getHostName: ssh not reachable yet. Retrying"
			v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
				"RetryingSSHConnection", clusterv1beta1.ConditionSeverityInfo,
				"%s", msg)
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineRetryingSSHConnectionV1Beta2Reason,
				Message: msg,
			})
			// Pure SSH retry, no hcloud API cost, so requeue immediately.
			return reconcile.Result{RequeueAfter: requeueImmediately}, nil
		}
		err = fmt.Errorf("get hostname failed: %w", err)
		s.scope.Error(err, "")
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"GetHostnameFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", err.Error())
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  infrav1.HCloudMachineGettingHostnameFailedV1Beta2Reason,
			Message: err.Error(),
		})
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	v1beta1conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:   infrav1.HCloudMachineServerCreatedV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineServerCreatedV1Beta2Reason,
	})

	remoteHostName := output.String()

	if remoteHostName != "rescue" {
		msg := fmt.Sprintf("Remote hostname (via ssh) of hcloud server is %q. Expected 'rescue'. Deleting hcloud machine", remoteHostName)
		s.scope.Error(nil, msg)
		err := s.scope.SetErrorAndRemediate(ctx, msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"UnexpectedHostname", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineUnexpectedHostnameV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{}, nil
	}

	// Now we know that we are inside a rescue system.
	// image-url-command has not started yet. Start it.

	data, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("hcloud GetRawBootstrapData failed: %w", err)
	}

	imageURLCommandPath, err := utils.ResolveImageURLCommandPath(hcloudImageURLCommandDir, hm.Spec.ImageURLCommand)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("resolving imageURLCommand failed: %w", err)
	}

	exitStatus, stdoutStderr, err := sshClient.StartImageURLCommand(ctx, imageURLCommandPath, hm.Spec.ImageURL, data, s.scope.Name(), []string{"sda"})
	if err != nil {
		err := fmt.Errorf("StartImageURLCommand failed (retrying): %w", err)
		// This could be a temporary network error. Retry.
		s.scope.Error(err, "",
			"ImageURLCommand", hm.Spec.ImageURLCommand,
			"exitStatus", exitStatus,
			"stdoutStderr", stdoutStderr)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"StartImageURLCommandFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", err.Error())
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineStartImageURLCommandFailedV1Beta2Reason,
			Message: err.Error(),
		})
		return reconcile.Result{}, err
	}

	if exitStatus != 0 {
		msg := "StartImageURLCommand failed with non-zero exit status. Deleting machine"
		s.scope.Error(nil, msg,
			"ImageURLCommand", hm.Spec.ImageURLCommand,
			"exitStatus", exitStatus,
			"stdoutStderr", stdoutStderr)
		err := s.scope.SetErrorAndRemediate(ctx, msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"StartImageURLCommandNoZeroExitCode", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineStartImageURLCommandNonZeroExitCodeV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{}, nil
	}

	v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
		"CustomProvisionerRunning", clusterv1beta1.ConditionSeverityInfo,
		"custom provisioner running")
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HCloudMachineCustomProvisionerRunningV1Beta2Reason,
		Message: "custom provisioner running",
	})
	s.setBootState(infrav1.HCloudBootStateRunningImageCommand)
	// The next state (RunningImageCommand) polls via SSH, which costs no hcloud API calls, but
	// the custom provisioner needs time to run, so wait a bit before the first attempt instead
	// of retrying immediately.
	return reconcile.Result{RequeueAfter: 20 * time.Second}, nil
}

// handleBootStateRunningImageCommand is for provisioning with imageURL and image-url-command.
func (s *Service) handleBootStateRunningImageCommand(ctx context.Context) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine

	hcloudSSHClient, err := s.getSSHClient(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getSSHClient failed (wait for image-url-command): %w", err)
	}

	state, logFile, err := hcloudSSHClient.StateOfImageURLCommand(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("StateOfImageURLCommand failed: %w", err)
	}

	durationOfState := time.Since(hm.Status.BootStateSince.Time)
	// Please keep the number (20) in sync with the docstring of ImageURL.
	if durationOfState > 20*time.Minute {
		// timeout. Something has failed.
		timeoutMsg := fmt.Sprintf("image URL command timed out, in this state since %s", durationOfState.Round(time.Second).String())

		v1beta1Reason := "RunningImageCommandTimedOut"
		v1beta1Msg := timeoutMsg
		if existing := v1beta1conditions.Get(hm, infrav1.ServerProvisionedCondition); existing != nil {
			v1beta1Reason = existing.Reason
			if existing.Message != "" {
				v1beta1Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		v1beta2Reason := infrav1.HCloudMachineRunningImageURLCommandTimedOutV1Beta2Reason
		v1beta2Msg := timeoutMsg
		if existing := v1beta2conditions.Get(hm, infrav1.HCloudMachineServerProvisionedV1Beta2Condition); existing != nil {
			v1beta2Reason = existing.Reason
			if existing.Message != "" {
				v1beta2Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		s.scope.Error(errors.New(v1beta2Msg), "", "logFile", logFile)
		err := s.scope.SetErrorAndRemediate(ctx, v1beta2Msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		record.Warn(hm, "ImageURLCommandFailed", v1beta2Msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			v1beta1Reason, clusterv1beta1.ConditionSeverityWarning,
			"%s", v1beta1Msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  v1beta2Reason,
			Message: v1beta2Msg,
		})
		return reconcile.Result{}, nil
	}

	sshClient := hcloudSSHClient
	switch state {
	case sshclient.ImageURLCommandStateRunning:
		outputJSON, err := sshClient.ReadOutputJSON(ctx)
		if err != nil {
			s.scope.Error(err, "failed to read output.json")
			// Pure SSH retry, no hcloud API cost, so requeue immediately.
			return reconcile.Result{RequeueAfter: requeueImmediately}, nil
		}
		msg := "custom provisioner running"

		// If outputJSON is empty, imageURLCommand is still running and output.json was
		// either not created yet, or the command does not create it at all.
		if outputJSON != "" {
			output, err := imageurlcommand.Parse(outputJSON)
			if err != nil {
				s.scope.Error(err, "failed to parse image URL command output")
				return reconcile.Result{RequeueAfter: requeueImmediately}, nil
			}

			if output.Message != "" {
				msg = output.Message
			}
		}

		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"CustomProvisionerRunning", clusterv1beta1.ConditionSeverityInfo, "%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineCustomProvisionerRunningV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil

	case sshclient.ImageURLCommandStateFinishedSuccessfully:
		// IMAGE_URL_DONE was found in the stdout.
		s.scope.Info("CustomProvisionerOutput", "logFile", logFile)

		outputJSON, err := sshClient.ReadOutputJSON(ctx)
		if err != nil {
			s.scope.Error(err, "failed to read output.json")
			return reconcile.Result{RequeueAfter: requeueImmediately}, nil
		}

		s.scope.Info("CustomProvisionerOutputJSON", "outputJSON", outputJSON)

		// The image got installed. Now reboot in the real operating system.
		// The reboot runs inside the operating system, so the hcloud server status stays
		// "running" the whole time. handleBootingToRealOS therefore moves on to
		// OperatingSystemRunning right away, usually while the reboot is still ongoing.
		// That is fine: the machine only becomes ready once the node has joined the
		// cluster, which CAPI checks, so nothing is gated on the reboot being finished.
		if rebootErr := hcloudSSHClient.Reboot(ctx).Err; rebootErr != nil {
			return reconcile.Result{}, fmt.Errorf("reboot after ImageURLCommand failed: %w", rebootErr)
		}

		s.setBootState(infrav1.HCloudBootStateBootingToRealOS)

		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"BootingToRealOS", clusterv1beta1.ConditionSeverityInfo,
			"Operating system of node is booting")
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineBootingToRealOSV1Beta2Reason,
			Message: "Operating system of node is booting",
		})

		return reconcile.Result{RequeueAfter: requeueImmediately}, nil

	case sshclient.ImageURLCommandStateFailed:
		s.scope.Error(nil, "custom provisioner failed", "logFile", logFile)

		outputJSON, err := sshClient.ReadOutputJSON(ctx)
		if err != nil {
			s.scope.Error(err, "failed to read output.json")
			return reconcile.Result{RequeueAfter: requeueImmediately}, nil
		}

		msg := "custom provisioner failed"
		if outputJSON != "" {
			output, err := imageurlcommand.Parse(outputJSON)
			if err != nil {
				s.scope.Error(err, "failed to parse output.json", "outputJSON", outputJSON)
				return reconcile.Result{}, fmt.Errorf("failed to parse: %w", err)
			}
			s.scope.Error(nil, "CustomProvisionerOutputJSON", "outputJSON", outputJSON)
			if output.Message != "" {
				msg = output.Message
			}
		}
		err = errors.New(msg)
		s.scope.Error(err, "", "logFile", logFile)
		err = s.scope.SetErrorAndRemediate(ctx, msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		record.Warn(hm, "CustomProvisionerFailed", msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"CustomProvisionerFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineCustomProvisionerFailedV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{}, nil

	case sshclient.ImageURLCommandStateNotStarted:
		return reconcile.Result{}, fmt.Errorf("image-url-command not started in BootState %q? Should not happen",
			state)

	default:
		return reconcile.Result{}, fmt.Errorf("unknown ImageURLCommandState: %q", state)
	}
}

// handleBootingToRealOS is used for both ways (imageName/snapshot and imageURL).
func (s *Service) handleBootingToRealOS(ctx context.Context) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine

	server, res, err := s.getLiveServer(ctx)
	if server == nil || err != nil || !res.IsZero() {
		return res, err
	}
	updateHCloudMachineStatusFromServer(hm, server)

	durationOfState := time.Since(hm.Status.BootStateSince.Time)
	if durationOfState > 6*time.Minute {
		// timeout. Something has failed.
		timeoutMsg := fmt.Sprintf("booting to real OS timed out, in this state since %s", durationOfState.Round(time.Second).String())

		v1beta1Reason := "BootingToRealOSTimedOut"
		v1beta1Msg := timeoutMsg
		if existing := v1beta1conditions.Get(hm, infrav1.ServerProvisionedCondition); existing != nil {
			v1beta1Reason = existing.Reason
			if existing.Message != "" {
				v1beta1Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		v1beta2Reason := infrav1.HCloudMachineBootingToRealOSTimedOutV1Beta2Reason
		v1beta2Msg := timeoutMsg
		if existing := v1beta2conditions.Get(hm, infrav1.HCloudMachineServerProvisionedV1Beta2Condition); existing != nil {
			v1beta2Reason = existing.Reason
			if existing.Message != "" {
				v1beta2Msg = fmt.Sprintf("%s (%s)", existing.Message, timeoutMsg)
			}
		}

		err := s.scope.SetErrorAndRemediate(ctx, v1beta2Msg)
		if err != nil {
			return reconcile.Result{}, err
		}
		s.scope.Error(nil, v1beta2Msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			v1beta1Reason, clusterv1beta1.ConditionSeverityWarning,
			"%s", v1beta1Msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  v1beta2Reason,
			Message: v1beta2Msg,
		})
		return reconcile.Result{}, nil
	}

	// analyze status of server
	switch server.Status {
	case hcloud.ServerStatusOff:
		return s.handleServerStatusOff(ctx, server)

	case hcloud.ServerStatusStarting, hcloud.ServerStatusInitializing:
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"BootingToRealOS", clusterv1beta1.ConditionSeverityInfo,
			"Operating system of node is booting")
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineBootingToRealOSV1Beta2Reason,
			Message: "Operating system of node is booting",
		})
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil

	case hcloud.ServerStatusRunning:
		s.setBootState(infrav1.HCloudBootStateOperatingSystemRunning)
		v1beta1conditions.MarkTrue(hm, infrav1.ServerProvisionedCondition)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineBootingToRealOSV1Beta2Reason,
			Message: fmt.Sprintf("hcloud server status: %s", server.Status),
		})
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:   infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HCloudMachineServerProvisionedV1Beta2Reason,
		})
		// Show changes in Status and go to next BootState.
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil

	default:
		msg := fmt.Sprintf("hcloud server status unknown: %s", server.Status)
		s.scope.Error(nil, msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"ServerStatusUnknown", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerStatusUnknownV1Beta2Reason,
			Message: msg,
		})
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

// handleOperatingSystemRunning is the final state. It is used for both ways (imageName/snapshot and imageURL).
func (s *Service) handleOperatingSystemRunning(ctx context.Context) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine

	server, res, err := s.getLiveServer(ctx)
	if server == nil || err != nil || !res.IsZero() {
		return res, err
	}
	updateHCloudMachineStatusFromServer(hm, server)

	// Clean up old Status fields
	hm.Status.ExternalIDs.ActionIDEnableRescueSystem = 0
	hm.Status.ExternalIDs.ActionIDCreateServer = 0

	v1beta1conditions.MarkTrue(hm, infrav1.ServerProvisionedCondition)
	// Provisioning is complete.
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:   infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineServerProvisionedV1Beta2Reason,
	})

	// check whether server is attached to the network
	if err := s.reconcileNetworkAttachment(ctx, server); err != nil {
		reterr := fmt.Errorf("failed to reconcile network attachment: %w", err)
		v1beta1conditions.MarkFalse(
			hm,
			infrav1.ServerAvailableCondition,
			infrav1.NetworkAttachFailedReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			reterr.Error(),
		)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineAttachingToNetworkFailedV1Beta2Reason,
			Message: reterr.Error(),
		})
		return res, reterr
	}

	// nothing to do any more for worker nodes
	if !s.scope.IsControlPlane() {
		v1beta1conditions.MarkTrue(hm, infrav1.ServerAvailableCondition)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:   infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HCloudMachineServerAvailableV1Beta2Reason,
		})
		s.scope.SetReady(true)
		return res, nil
	}

	// all control planes have to be attached to the load balancer if it exists
	res, err = s.reconcileLoadBalancerAttachment(ctx, server)
	if err != nil {
		reterr := fmt.Errorf("failed to reconcile load balancer attachment: %w", err)
		v1beta1conditions.MarkFalse(
			hm,
			infrav1.ServerAvailableCondition,
			infrav1.LoadBalancerAttachFailedReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			reterr.Error(),
		)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineAttachingToLoadBalancerFailedV1Beta2Reason,
			Message: reterr.Error(),
		})
		return res, reterr
	}

	// Order matters:
	// 1. SetReady(true) first. This is what makes the Machine become ready and
	//    lets the Node get linked to it. Otherwise we deadlock:
	//    reconcileLoadBalancerAttachment only adds this control plane to the
	//    load balancer once its apiserver pod is marked healthy, and that can
	//    only happen after the Node is linked to the Machine, which in turn
	//    requires this call to SetReady.
	// 2. Return early on a non-zero res so the False reason set on
	//    ServerAvailable inside reconcileLoadBalancerAttachment is not overwritten.
	// 3. Mark ServerAvailable=True only on the happy path.
	s.scope.SetReady(true)
	if res != (reconcile.Result{}) {
		return res, nil
	}

	v1beta1conditions.MarkTrue(hm, infrav1.ServerAvailableCondition)
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:   infrav1.HCloudMachineServerAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineServerAvailableV1Beta2Reason,
	})
	return reconcile.Result{}, nil
}

// getLiveServer fetches the live hcloud server. It is only called by the two states
// (BootingToRealOS, OperatingSystemRunning) that need a server.Status transition or the
// network/LB attachment reconcile. Unset does not call this because the server does not exist
// yet. Initializing, EnablingRescue, BootingToRescue and RunningImageCommand drive their progress
// via GetAction polling and/or SSH, so they never call this and avoid an hcloud API call on every
// reconcile while they wait.
//
// Unless it returns a live server together with an empty res and a nil err, the caller must return
// (res, err) immediately. Some stop-paths (invalid token, rate limit, deleted server) deliberately
// return an empty res and a nil err, so a nil server is a stop signal in its own right.
func (s *Service) getLiveServer(ctx context.Context) (server *hcloud.Server, res reconcile.Result, err error) {
	server, err = s.findServer(ctx)
	if err != nil {
		if handleUnauthorized(s.scope.HCloudMachine, err) {
			return nil, reconcile.Result{}, nil
		}

		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			if !s.scope.HCloudMachine.Status.Ready {
				hcloudutil.HandleRateLimitExceededV1Beta1(s.scope.HCloudMachine, err, "findServer")
				return nil, reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return nil, reconcile.Result{}, nil
		}

		return nil, reconcile.Result{}, fmt.Errorf("findServer: %w", err)
	}
	markHCloudTokenAvailable(s.scope.HCloudMachine)

	// findServer returns nil for both server and error if the server was not found.
	if server == nil {
		// The server no longer exists in HCloud, it was deleted.
		// We set MachineError. CAPI will delete machine.
		msg := fmt.Sprintf("hcloud server (%q) no longer available. Setting MachineError.",
			*s.scope.HCloudMachine.Spec.ProviderID)

		s.scope.Error(errors.New(msg), msg)

		if err := s.scope.SetErrorAndRemediate(ctx, msg); err != nil {
			return nil, reconcile.Result{}, fmt.Errorf("SetErrorAndRemediate failed: %w", err)
		}
		record.Warn(s.scope.HCloudMachine, "NoHCloudServerFound", msg)
		v1beta1conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
			"NoHCloudServerFound", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerNotFoundV1Beta2Reason,
			Message: msg,
		})
		// no need to requeue.
		return nil, reconcile.Result{}, nil
	}

	return server, reconcile.Result{}, nil
}

// markHCloudTokenAvailable marks the HCloudTokenAvailableCondition as true. Call it after an
// hcloud API call succeeds, so the condition reflects the outcome of the most recent call.
func markHCloudTokenAvailable(hm *infrav1.HCloudMachine) {
	v1beta1conditions.MarkTrue(hm, infrav1.HCloudTokenAvailableCondition)
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:   infrav1.HCloudTokenAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudTokenAvailableV1Beta2Reason,
	})
}

// handleUnauthorized marks the HCloudTokenAvailableCondition as false if err is the "wrong
// hcloud token" error, and reports whether the caller should stop reconciling instead of
// retrying - there is no point retrying with invalid credentials.
func handleUnauthorized(hm *infrav1.HCloudMachine, err error) bool {
	if !errors.Is(err, hcloudclient.ErrUnauthorized) {
		return false
	}
	v1beta1conditions.MarkFalse(
		hm,
		infrav1.HCloudTokenAvailableCondition,
		infrav1.HCloudCredentialsInvalidReason,
		clusterv1beta1.ConditionSeverityError,
		"wrong hcloud token",
	)
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
		Message: "wrong hcloud token",
	})
	return true
}

// implements setting rate limit on hcloudmachine.
func handleRateLimit(hm *infrav1.HCloudMachine, err error, functionName string, errMsg string) error {
	// returns error if not a rate limit exceeded error
	if !hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	// does not return error if machine is running and does not have a deletion timestamp
	if hm.Status.Ready && hm.DeletionTimestamp.IsZero() {
		return nil
	}

	// check for a rate limit exceeded error if the machine is not running or if machine has a deletion timestamp
	hcloudutil.HandleRateLimitExceededV1Beta1(hm, err, functionName)
	return fmt.Errorf("%s: %w", errMsg, err)
}

// Delete implements delete method of server.
func (s *Service) Delete(ctx context.Context) (reconcile.Result, error) {
	// Set phase to deleting.
	s.scope.HCloudMachine.Status.InstanceState = ptr.To(hcloud.ServerStatusDeleting)
	v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
		Type:   infrav1.HCloudMachineServerAvailableV1Beta2Condition,
		Status: metav1.ConditionFalse,
		Reason: infrav1.HCloudMachineDeletingV1Beta2Reason,
	})

	// Nothing to do if ProviderID was never set.
	if s.scope.HCloudMachine.Spec.ProviderID == nil {
		return reconcile.Result{}, nil
	}

	server, err := s.findServer(ctx)
	if err != nil {
		// If it is an unauthorized error i.e. wrong HCloudToken do not return an error.
		// As there is no point retrying with invalid credentials.
		if handleUnauthorized(s.scope.HCloudMachine, err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "findServer", "failed to find server for deletion")
	}
	markHCloudTokenAvailable(s.scope.HCloudMachine)

	// if no server has been found, then nothing can be deleted
	if server == nil {
		providerID := "nil"
		if s.scope.HCloudMachine.Spec.ProviderID != nil {
			providerID = *s.scope.HCloudMachine.Spec.ProviderID
		}
		msg := fmt.Sprintf("Unable to delete HCloud server. Could not find matching server for %s. ProviderID: %q", s.scope.Name(), providerID)
		s.scope.V(1).Info(msg)
		record.Warn(s.scope.HCloudMachine, "NoInstanceFound", msg)
		return reconcile.Result{}, nil
	}

	// control planes have to be deleted as targets of server
	if s.scope.IsControlPlane() && s.scope.HetznerCluster.Spec.ControlPlaneLoadBalancer.Enabled {
		for _, target := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
			if target.Type == infrav1.LoadBalancerTargetTypeServer && target.ServerID == server.ID {
				if err := s.deleteServerOfLoadBalancer(ctx, server); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to delete attached server of loadbalancer: %w", err)
				}
				break
			}
		}
	}

	updateHCloudMachineStatusFromServer(s.scope.HCloudMachine, server)

	// first shut the server down, then delete it
	switch server.Status {
	case hcloud.ServerStatusOff:
		return s.handleDeleteServerStatusOff(ctx, server)
	default:
		return s.handleDeleteServerStatusRunning(ctx, server)
	}
}

func (s *Service) reconcileNetworkAttachment(ctx context.Context, server *hcloud.Server) error {
	// if no network exists, then do nothing
	if s.scope.HetznerCluster.Status.Network == nil {
		return nil
	}

	// if it is already attached to network, then do nothing
	for _, id := range s.scope.HetznerCluster.Status.Network.AttachedServers {
		if id == server.ID {
			return nil
		}
	}

	// attach server to network
	if err := s.scope.HCloudClient.AttachServerToNetwork(ctx, server, hcloud.ServerAttachToNetworkOpts{
		Network: &hcloud.Network{
			ID: s.scope.HetznerCluster.Status.Network.ID,
		},
	}); err != nil {
		// check if network status is old and server is in fact already attached
		if hcloud.IsError(err, hcloud.ErrorCodeServerAlreadyAttached) {
			return nil
		}
		return handleRateLimit(s.scope.HCloudMachine, err, "AttachServerToNetwork", "failed to attach server to network")
	}

	return nil
}

func (s *Service) reconcileLoadBalancerAttachment(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine

	if s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer == nil {
		return reconcile.Result{}, nil
	}

	// remove the server as soon as the Machine starts deleting.
	if !s.scope.Machine.DeletionTimestamp.IsZero() {
		if err := s.deleteServerOfLoadBalancer(ctx, server); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to delete server %s with ID %d from loadbalancer: %w", server.Name, server.ID, err)
		}
		return reconcile.Result{}, nil
	}

	// if already attached do nothing
	for _, target := range s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target {
		if target.Type == infrav1.LoadBalancerTargetTypeServer && target.ServerID == server.ID {
			return reconcile.Result{}, nil
		}
	}

	// we differentiate between private and public net
	var hasPrivateIP bool
	if len(server.PrivateNet) > 0 {
		hasPrivateIP = true
	}

	// if load balancer has not been attached to a network, then it cannot add a server with private IP
	if hasPrivateIP && v1beta1conditions.IsFalse(s.scope.HetznerCluster, infrav1.LoadBalancerReadyCondition) {
		return reconcile.Result{}, nil
	}

	// attach only if server has private IP or public IPv4, otherwise Hetzner cannot handle it
	if server.PublicNet.IPv4.IP == nil && !hasPrivateIP {
		return reconcile.Result{}, nil
	}

	apiServerPodHealthy := !s.scope.Cluster.Spec.ControlPlaneRef.IsDefined() ||
		s.scope.Cluster.Spec.ControlPlaneRef.Kind != "KubeadmControlPlane" ||
		conditions.IsTrue(s.scope.Machine, controlplanev1.KubeadmControlPlaneMachineAPIServerPodHealthyCondition)

	// we attach only nodes with kube-apiserver pod healthy to avoid downtime, skipped for the first node
	if len(s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target) > 0 && !apiServerPodHealthy {
		v1beta1conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			"WaitingForAPIServer", clusterv1beta1.ConditionSeverityInfo,
			"reconcile LoadBalancer: apiserver pod not healthy yet.")
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineWaitingForAPIServerV1Beta2Reason,
			Message: "reconcile LoadBalancer: apiserver pod not healthy yet.",
		})
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	opts := hcloud.LoadBalancerAddServerTargetOpts{
		Server:       server,
		UsePrivateIP: &hasPrivateIP,
	}
	loadBalancer := &hcloud.LoadBalancer{
		ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID,
	}

	if err := s.scope.HCloudClient.AddTargetServerToLoadBalancer(ctx, opts, loadBalancer); err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeTargetAlreadyDefined) {
			return reconcile.Result{}, nil
		}
		errMsg := fmt.Sprintf("failed to add server %s with ID %d as target to load balancer", server.Name, server.ID)
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "AddTargetServerToLoadBalancer", errMsg)
	}

	record.Eventf(
		s.scope.HetznerCluster,
		"AddedAsTargetToLoadBalancer",
		"Added new server %s with ID %d to the loadbalancer with ID %d",
		server.Name, server.ID, s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID)

	return reconcile.Result{}, nil
}

func (s *Service) createServerFromImageNameOrURL(ctx context.Context) (*hcloud.Server, *hcloud.Image, error) {
	if s.scope.HCloudMachine.Spec.ImageName != "" {
		return s.createServerFromImageName(ctx)
	}
	return s.createServerFromImageURL(ctx)
}

func (s *Service) createServerFromImageURL(ctx context.Context) (*hcloud.Server, *hcloud.Image, error) {
	hm := s.scope.HCloudMachine

	// This is a new machine with imageURL. The webhook validates that ImageURLCommand is set
	// when ImageURL is set, and rejects any name that does not match the basename pattern. We
	// still resolve the path at runtime so an empty or invalid name (for example, if the webhook
	// has been disabled temporarily) is rejected before we copy anything into the rescue system.
	imageURLCommandName := hm.Spec.ImageURLCommand
	if _, err := utils.ResolveImageURLCommandPath(hcloudImageURLCommandDir, imageURLCommandName); err != nil {
		err = fmt.Errorf("imageURLCommand %q is invalid or not accessible by the controller pod: %w", imageURLCommandName, err)
		s.scope.Error(err, "")
		v1beta1conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerProvisionedCondition,
			"ImageURLCommandNotAccessible", clusterv1beta1.ConditionSeverityWarning,
			"%s", err.Error())
		return nil, nil, errServerCreateStopReconcile
	}

	image, err := s.getServerImage(ctx, preRescueOSImage)
	if err != nil {
		err = fmt.Errorf("failed to get pre-rescue-OS server image %q: %w", preRescueOSImage, err)
		msg := err.Error()
		record.Warn(hm, "FailedGetServerImage", msg)
		s.scope.Error(nil, msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"GetServerImageFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineGettingServerImageFailedV1Beta2Reason,
			Message: msg,
		})
		return nil, nil, err
	}

	// Create the server powered off. Enabling the rescue system while the server is off means
	// its first boot goes directly into the rescue system, and the pre-rescue-OS image never boots.
	result, err := s.createServer(ctx, nil, image, false)
	if err != nil {
		return nil, nil, err
	}

	// handleBootStateInitializing waits for this action before enabling the rescue system.
	hm.Status.ExternalIDs.ActionIDCreateServer = result.Action.ID

	s.setBootState(infrav1.HCloudBootStateInitializing)
	return result.Server, image, nil
}

func (s *Service) createServerFromImageName(ctx context.Context) (*hcloud.Server, *hcloud.Image, error) {
	hm := s.scope.HCloudMachine
	userData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get raw bootstrap data: %s", err)
		msg := err.Error()
		record.Warn(hm, "FailedGetBootstrapData", msg)
		s.scope.Error(nil, msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"GetRawBootstrapDataFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineGettingRawBootstrapDataFailedV1Beta2Reason,
			Message: msg,
		})
		return nil, nil, err
	}

	image, err := s.getServerImage(ctx, hm.Spec.ImageName)
	if err != nil {
		err = fmt.Errorf("create server from imageName (%q): %w", hm.Spec.ImageName, err)
		msg := err.Error()
		record.Warn(hm, "FailedGetServerImage", msg)
		s.scope.Error(nil, msg)
		v1beta1conditions.MarkFalse(hm, infrav1.ServerProvisionedCondition,
			"GetServerImageFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineGettingServerImageFailedV1Beta2Reason,
			Message: msg,
		})
		return nil, nil, err
	}

	// The imageName flow does not use the rescue system, so the server can start right after create.
	result, err := s.createServer(ctx, userData, image, true)
	if err != nil {
		return nil, nil, err
	}

	s.setBootState(infrav1.HCloudBootStateBootingToRealOS)
	return result.Server, image, nil
}

func (s *Service) createServer(ctx context.Context, userData []byte, image *hcloud.Image, startAfterCreate bool) (hcloud.ServerCreateResult, error) {
	hm := s.scope.HCloudMachine
	automount := false
	opts := hcloud.ServerCreateOpts{
		Name:   s.scope.Name(),
		Labels: s.createLabels(),
		Image:  image,
		Location: &hcloud.Location{
			Name: string(hm.Status.Region),
		},
		ServerType: &hcloud.ServerType{
			Name: string(hm.Spec.Type),
		},
		Automount:        &automount,
		StartAfterCreate: &startAfterCreate,
		UserData:         string(userData),
		PublicNet: &hcloud.ServerCreatePublicNet{
			EnableIPv4: hm.Spec.PublicNetwork.EnableIPv4,
			EnableIPv6: hm.Spec.PublicNetwork.EnableIPv6,
		},
	}

	// set placement group if necessary
	if hm.Spec.PlacementGroupName != nil {
		var foundPlacementGroupInStatus bool
		for _, pgSts := range s.scope.HetznerCluster.Status.HCloudPlacementGroups {
			if *hm.Spec.PlacementGroupName == pgSts.Name {
				foundPlacementGroupInStatus = true
				opts.PlacementGroup = &hcloud.PlacementGroup{
					ID:   pgSts.ID,
					Name: pgSts.Name,
					Type: hcloud.PlacementGroupType(pgSts.Type),
				}
			}
		}
		if !foundPlacementGroupInStatus {
			msg := fmt.Sprintf("Placement group %q does not exist in cluster",
				*hm.Spec.PlacementGroupName)
			v1beta1conditions.MarkFalse(hm,
				infrav1.ServerCreateSucceededCondition,
				infrav1.InstanceHasNonExistingPlacementGroupReason,
				clusterv1beta1.ConditionSeverityError,
				"%s", msg,
			)
			v1beta2conditions.Set(hm, metav1.Condition{
				Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineServerPlacementGroupNotFoundV1Beta2Reason,
				Message: msg,
			})
			return hcloud.ServerCreateResult{}, fmt.Errorf("%s: %w", msg, errServerCreateNotPossible)
		}
	}

	caphSSHKeys, hcloudSSHKeys, err := s.getSSHKeys(ctx)
	if err != nil {
		return hcloud.ServerCreateResult{}, err
	}
	opts.SSHKeys = hcloudSSHKeys

	// set up network if available
	if net := s.scope.HetznerCluster.Status.Network; net != nil {
		opts.Networks = []*hcloud.Network{{
			ID: net.ID,
		}}
	}

	// if no private network exists, there must be an IPv4 for the load balancer
	if !s.scope.HetznerCluster.Spec.HCloudNetwork.Enabled {
		opts.PublicNet.EnableIPv4 = true
	}

	// Create the server
	result, err := s.scope.HCloudClient.CreateServer(ctx, opts)
	if err != nil {
		serverType := "nil"
		if opts.ServerType != nil {
			serverType = opts.ServerType.Name
		}

		msg := fmt.Sprintf("failed to create HCloud server %q in %q (type %q)",
			hm.Name, opts.Location.Name, serverType)

		if hcloudutil.HandleRateLimitExceededV1Beta1(hm, err, "CreateServer") {
			// RateLimit was reached. Condition and Event got already created.
			return hcloud.ServerCreateResult{}, fmt.Errorf("%s: %w", msg, err)
		}

		// A server with this exact name already exists. This happens if a previous reconcile
		// created the HCloud server successfully but lost the update that would have persisted
		// ProviderID/BootState (e.g. an API server conflict or a controller restart between
		// CreateServer and Close()). Adopt the existing server instead of failing forever with
		// the same uniqueness error on every retry.
		if hcloud.IsError(err, hcloud.ErrorCodeUniquenessError) {
			existingServer, findErr := s.findServerByName(ctx)
			if findErr != nil {
				s.scope.Error(findErr, "failed to look up existing server after a uniqueness error on CreateServer")
			} else if existingServer != nil {
				s.scope.Info("server already exists after a uniqueness error, adopting it instead of failing",
					"serverID", existingServer.ID, "serverName", existingServer.Name)
				hm.Status.SSHKeys = caphSSHKeys
				v1beta1conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)
				v1beta2conditions.Set(hm, metav1.Condition{
					Type:   infrav1.HCloudMachineServerCreatedV1Beta2Condition,
					Status: metav1.ConditionTrue,
					Reason: infrav1.HCloudMachineServerCreatedV1Beta2Reason,
				})
				record.Eventf(hm, "AdoptedExistingServer", "Adopted existing server %s (ID %d) after a uniqueness error on create",
					existingServer.Name, existingServer.ID)
				return hcloud.ServerCreateResult{Server: existingServer, Action: &hcloud.Action{ID: actionDone}}, nil
			}
		}

		msg = fmt.Sprintf("%s: %s", msg, err.Error())
		if hcloud.IsError(err, hcloud.ErrorCodeUniquenessError) {
			// The name is taken and we could not adopt the existing server. Give the operator the
			// concrete fix instead of the raw uniqueness error.
			msg = fmt.Sprintf(
				"Server creation failed because a server named %q already exists and it could not be adopted automatically: %s. "+
					"Delete the conflicting HCloud server, or delete this Machine to get a replacement with a new name (deleting the Machine object leaves the original server behind as a dangling server). ",
				hm.Name, err.Error())
		}
		s.scope.Error(nil, msg)
		// No condition was set yet. Set a general condition to false.
		v1beta1conditions.MarkFalse(hm, infrav1.ServerCreateSucceededCondition,
			infrav1.ServerCreateFailedReason, clusterv1beta1.ConditionSeverityWarning, "%s", msg)
		v1beta2conditions.Set(hm, metav1.Condition{
			Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerCreationFailedV1Beta2Reason,
			Message: msg,
		})
		record.Warn(hm, "FailedCreateHCloudServer", msg)
		return hcloud.ServerCreateResult{}, handleRateLimit(hm, err, "CreateServer", msg)
	}

	// set ssh keys to status
	hm.Status.SSHKeys = caphSSHKeys

	v1beta1conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)
	v1beta2conditions.Set(hm, metav1.Condition{
		Type:   infrav1.HCloudMachineServerCreatedV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudMachineServerCreatedV1Beta2Reason,
	})
	record.Eventf(hm, "SuccessfulCreate", "Created new server %s with ID %d", result.Server.Name, result.Server.ID)
	return result, nil
}

// getSSHKeys collects the set of SSH keys to use when creating a server in Hetzner Cloud,
// and validates that they exist in the HCloud API.
//
// The function:
//  1. Starts with the SSH keys defined in HCloudMachine.Spec.SSHKeys.
//     If none are defined there, it falls back to HetznerCluster.Spec.SSHKeys.HCloud.
//  2. Always adds the SSH key referenced in the Hetzner secret (if present),
//     ensuring it is included even if not listed in the spec.
//  3. Fetches the complete list of SSH keys stored in HCloud via the API.
//  4. Verifies that every SSH key referenced in the spec or secret exists in HCloud.
//     If any key is missing, it updates machine conditions and returns an error.
//  5. Builds and returns two slices:
//     - caphSSHKeys: the logical set of SSH keys referenced in the spec/secret,
//     suitable for storing in the HCloudMachine status.
//     - hcloudSSHKeys: the corresponding HCloud API objects, suitable for passing
//     to the HCloud CreateServer API call.
func (s *Service) getSSHKeys(ctx context.Context) (
	caphSSHKeys []infrav1.SSHKey,
	hcloudSSHKeys []*hcloud.SSHKey,
	reterr error,
) {
	caphSSHKeys = s.scope.HCloudMachine.Spec.SSHKeys

	// if no ssh keys are specified on the machine, take the ones from the cluster
	if len(caphSSHKeys) == 0 {
		caphSSHKeys = s.scope.HetznerCluster.Spec.SSHKeys.HCloud
	}

	// always add ssh key from secret if one is found
	sshKeyName := s.scope.HetznerSecret().Data[s.scope.HetznerCluster.Spec.HetznerSecret.Key.SSHKey]
	if len(sshKeyName) > 0 {
		// Check if the SSH key name already exists
		keyExists := false
		for _, key := range caphSSHKeys {
			if string(sshKeyName) == key.Name {
				keyExists = true
				break
			}
		}

		// If the SSH key name doesn't exist, append it
		if !keyExists {
			caphSSHKeys = append(caphSSHKeys, infrav1.SSHKey{Name: string(sshKeyName)})
		}
	}

	// get all ssh keys that are stored in HCloud API
	allHcloudSSHKeys, err := s.scope.HCloudClient.ListSSHKeys(ctx, hcloud.SSHKeyListOpts{})
	if err != nil {
		return nil, nil, handleRateLimit(s.scope.HCloudMachine, err, "ListSSHKeys", "failed listing ssh keys from hcloud")
	}

	// Create a map, so we can easily check if each caphSSHKey exist in HCloud.
	sshKeysAPIMap := make(map[string]*hcloud.SSHKey, len(allHcloudSSHKeys))
	for i, sshKey := range allHcloudSSHKeys {
		sshKeysAPIMap[sshKey.Name] = allHcloudSSHKeys[i]
	}

	// Check caphSSHKeys. Fail if key is not in HCloud
	for _, sshKeySpec := range caphSSHKeys {
		sshKey, ok := sshKeysAPIMap[sshKeySpec.Name]
		if !ok {
			msg := fmt.Sprintf("ssh key %q not present in hcloud", sshKeySpec.Name)
			s.scope.Error(nil, msg)
			v1beta1conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.ServerCreateSucceededCondition,
				infrav1.SSHKeyNotFoundReason,
				clusterv1beta1.ConditionSeverityError,
				"%s", msg)
			v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
				Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineServerSSHKeyNotFoundV1Beta2Reason,
				Message: msg,
			})
			return nil, nil, fmt.Errorf("%s: %w", msg, errServerCreateNotPossible)
		}
		hcloudSSHKeys = append(hcloudSSHKeys, sshKey)
	}

	return caphSSHKeys, hcloudSSHKeys, nil
}

func (s *Service) getServerImage(ctx context.Context, imageName string) (*hcloud.Image, error) {
	key := fmt.Sprintf("%s%s", infrav1.NameHetznerProviderPrefix, "image-name")

	// Get server type so we can filter for images with correct architecture
	serverType, err := s.scope.HCloudClient.GetServerType(ctx, string(s.scope.HCloudMachine.Spec.Type))
	if err != nil {
		// If it is an unauthorized error i.e. wrong HCloudToken, set HCloudCredentialsInvalid condition.
		if errors.Is(err, hcloudclient.ErrUnauthorized) {
			v1beta1conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.HCloudTokenAvailableCondition,
				infrav1.HCloudCredentialsInvalidReason,
				clusterv1beta1.ConditionSeverityError,
				"wrong hcloud token",
			)
			v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: "wrong hcloud token",
			})
			return nil, err
		}

		return nil, handleRateLimit(s.scope.HCloudMachine, err, "GetServerType", "failed to get server type in HCloud")
	}

	v1beta1conditions.MarkTrue(s.scope.HCloudMachine, infrav1.HCloudTokenAvailableCondition)
	v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
		Type:   infrav1.HCloudTokenAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HCloudTokenAvailableV1Beta2Reason,
	})

	if serverType == nil {
		msg := fmt.Sprintf("failed to get server type %q", string(s.scope.HCloudMachine.Spec.Type))
		v1beta1conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ServerTypeNotFoundReason,
			clusterv1beta1.ConditionSeverityError,
			"%s", msg,
		)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerTypeNotFoundV1Beta2Reason,
			Message: msg,
		})
		return nil, fmt.Errorf("%s: %w", msg, errServerCreateNotPossible)
	}

	// query for an existing image by label
	// this is needed because snapshots don't have a name, only descriptions and labels
	listOpts := hcloud.ImageListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: fmt.Sprintf("%s==%s", key, imageName),
		},
		Architecture: []hcloud.Architecture{serverType.Architecture},
	}

	images, err := s.scope.HCloudClient.ListImages(ctx, listOpts)
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "ListImages", "failed to list images by label in HCloud")
	}

	// query for an existing image by name.
	listOpts = hcloud.ImageListOpts{
		Name:         imageName,
		Architecture: []hcloud.Architecture{serverType.Architecture},
	}
	imagesByName, err := s.scope.HCloudClient.ListImages(ctx, listOpts)
	if err != nil {
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "ListImages", "failed to list images by name in HCloud")
	}

	images = append(images, imagesByName...)

	if len(images) > 1 {
		msg := fmt.Sprintf("image is ambiguous - %d images have name %s",
			len(images), imageName)
		record.Warn(s.scope.HCloudMachine, "ImageNameAmbiguous", msg)
		v1beta1conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ImageAmbiguousReason,
			clusterv1beta1.ConditionSeverityError,
			"%s", msg,
		)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerImageAmbiguousV1Beta2Reason,
			Message: msg,
		})
		return nil, fmt.Errorf("%s: %w", msg, errServerCreateNotPossible)
	}
	if len(images) == 0 {
		msg := fmt.Sprintf("no image found with name %s", s.scope.HCloudMachine.Spec.ImageName)
		record.Warn(s.scope.HCloudMachine, "ImageNotFound", msg)
		v1beta1conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ImageNotFoundReason,
			clusterv1beta1.ConditionSeverityError,
			"%s", msg,
		)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerCreatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerImageNotFoundV1Beta2Reason,
			Message: msg,
		})
		return nil, fmt.Errorf("%s: %w", msg, errServerCreateNotPossible)
	}

	return images[0], nil
}

// handleServerStatusOff is only called from handleBootingToRealOS (pre-provisioning).
// If this function is ever called post-provisioning, it should set ServerAvailable instead of ServerProvisioned.
func (s *Service) handleServerStatusOff(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// Check if server is in ServerStatusOff and turn it on. This is to avoid a bug of Hetzner where
	// sometimes machines are created and not turned on

	serverProvisionedCondition := v1beta1conditions.Get(s.scope.HCloudMachine, infrav1.ServerProvisionedCondition)
	if serverProvisionedCondition != nil &&
		serverProvisionedCondition.Status == corev1.ConditionFalse &&
		serverProvisionedCondition.Reason == infrav1.ServerOffReason {
		s.scope.Info("Trigger power on again")
		if time.Now().Before(serverProvisionedCondition.LastTransitionTime.Add(serverOffTimeout)) {
			// Not yet timed out, try again to power on
			if err := s.scope.HCloudClient.PowerOnServer(ctx, server); err != nil {
				if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
					// if server is locked, we just retry again
					v1beta1conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerProvisionedCondition,
						"PowerOnServerFailed", clusterv1beta1.ConditionSeverityInfo,
						"handleServerStatusOff: server locked. Will retry")
					v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
						Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
						Status:  metav1.ConditionFalse,
						Reason:  infrav1.HCloudMachinePoweringOnServerFailedV1Beta2Reason,
						Message: "handleServerStatusOff: server locked. Will retry",
					})
					return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
				}
				return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "PowerOnServer", "failed to power on server")
			}
		} else {
			// Timed out. Set failure reason
			err := s.scope.SetErrorAndRemediate(ctx, "reached timeout of waiting for machines that are switched off")
			if err != nil {
				return reconcile.Result{}, err
			}
			v1beta1conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerProvisionedCondition,
				"ServerOffTimeout", clusterv1beta1.ConditionSeverityWarning,
				"reached timeout waiting for server that is switched off")
			v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
				Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineServerOffTimeoutV1Beta2Reason,
				Message: "reached timeout waiting for server that is switched off",
			})
			return res, nil
		}
	} else {
		// No condition set yet. Try to power server on.
		if err := s.scope.HCloudClient.PowerOnServer(ctx, server); err != nil {
			if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
				// if server is locked, we just retry again
				v1beta1conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerProvisionedCondition,
					"PowerOnServerFailed", clusterv1beta1.ConditionSeverityInfo, "handleServerStatusOff: server locked. Will retry")
				v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
					Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav1.HCloudMachinePoweringOnServerFailedV1Beta2Reason,
					Message: "handleServerStatusOff: server locked. Will retry",
				})
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "PowerOnServer", "failed to power on server")
		}
		v1beta1conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerProvisionedCondition,
			infrav1.ServerOffReason,
			clusterv1beta1.ConditionSeverityInfo,
			"server is switched off",
		)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerProvisionedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineServerOffV1Beta2Reason,
			Message: "server is switched off",
		})
	}

	// Try again in 30 sec.
	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

func (s *Service) handleDeleteServerStatusRunning(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// Shut down the server if one of the two conditions apply:
	// 1. The server has not yet been tried to shut down and still is marked as "ready".
	// 2. The server has been tried to shut down without an effect and the timeout is not reached yet.

	if s.scope.HasServerAvailableCondition() {
		if err := s.scope.HCloudClient.ShutdownServer(ctx, server); err != nil {
			return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "ShutdownServer", "failed to shutdown server")
		}

		v1beta1conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.ServerTerminatingReason,
			clusterv1beta1.ConditionSeverityInfo,
			"Instance has been shut down",
		)
		v1beta2conditions.Set(s.scope.HCloudMachine, metav1.Condition{
			Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HCloudMachineDeletingV1Beta2Reason,
			Message: "Instance has been shut down",
		})

		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// timeout for shutdown has been reached - delete server
	if err := s.scope.HCloudClient.DeleteServer(ctx, server); err != nil {
		record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "DeleteServer", "failed to delete server")
	}

	record.Eventf(s.scope.HCloudMachine, "HCloudServerDeleted", "HCloud server %s deleted", s.scope.Name())
	return res, nil
}

func (s *Service) handleDeleteServerStatusOff(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// server is off and can be deleted
	if err := s.scope.HCloudClient.DeleteServer(ctx, server); err != nil {
		record.Warnf(s.scope.HCloudMachine, "FailedDeleteHCloudServer", "Failed to delete HCloud server %s", s.scope.Name())
		return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "DeleteServer", "failed to delete server")
	}

	record.Eventf(s.scope.HCloudMachine, "HCloudServerDeleted", "HCloud server %s deleted", s.scope.Name())
	return res, nil
}

func (s *Service) deleteServerOfLoadBalancer(ctx context.Context, server *hcloud.Server) error {
	lb := &hcloud.LoadBalancer{ID: s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.ID}

	if err := s.scope.HCloudClient.DeleteTargetServerOfLoadBalancer(ctx, lb, server); err != nil {
		// Do not return an error in case the target server was not found.
		// In case the target server was not found we will get an error similar to
		// "server with ID xxxxx not found (invalid_input, xxxxxxx)".
		// If the load balancer itself was not found then we will get a "not_found" error.
		// In both cases, don't do anything.
		if hcloud.IsError(err, hcloud.ErrorCodeInvalidInput) || hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			return nil
		}

		errMsg := fmt.Sprintf("failed to delete server %s with ID %d as target of load balancer %s with ID %d", server.Name, server.ID, lb.Name, lb.ID)
		return handleRateLimit(s.scope.HCloudMachine, err, "DeleteTargetServerOfLoadBalancer", errMsg)
	}
	record.Eventf(
		s.scope.HetznerCluster,
		"DeletedTargetOfLoadBalancer",
		"Deleted new server %s with ID %d of the loadbalancer %s with ID %d",
		server.Name, server.ID, lb.Name, lb.ID,
	)

	return nil
}

// findServer attempts to locate the HCloud server for the underlying HCloudMachine.
// It first tries to find the server by its provider ID. If that fails (e.g., provider ID not yet set),
// it falls back to searching by labels.
//
// It returns server and error as nil when the server is not found because hcloud-go's GetServer returns nil
// for a non-existent server ID and no server matched the label selector.
func (s *Service) findServer(ctx context.Context) (*hcloud.Server, error) {
	var server *hcloud.Server

	// try to find the server based on its id
	serverID, err := s.scope.ServerIDFromProviderID()
	if err == nil {
		server, err = s.scope.HCloudClient.GetServer(ctx, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to get server %d: %w", serverID, err)
		}

		// if server has been found, return it
		if server != nil {
			return server, nil
		}
	}

	// server has not been found via id - try to find the server based on its labels
	opts := hcloud.ServerListOpts{}

	opts.LabelSelector = utils.LabelsToLabelSelector(s.createLabels())

	servers, err := s.scope.HCloudClient.ListServers(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	if len(servers) > 1 {
		err := fmt.Errorf("found %d servers with name %s", len(servers), s.scope.Name())
		record.Warn(s.scope.HCloudMachine, "MultipleInstances", err.Error())
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	s.scope.Info("DeprecationWarning finding Server by labels is no longer needed. We plan to remove that feature and rename findServer to getServer", "err", err)

	return servers[0], nil
}

// findServerByName searches for a server with this HCloudMachine's exact name. Used to recover
// from a uniqueness error on CreateServer, where relying on ProviderID isn't possible yet.
// It returns server and error as nil when no server matches.
func (s *Service) findServerByName(ctx context.Context) (*hcloud.Server, error) {
	servers, err := s.scope.HCloudClient.ListServers(ctx, hcloud.ServerListOpts{Name: s.scope.Name()})
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	if len(servers) > 1 {
		return nil, fmt.Errorf("found %d servers with name %s", len(servers), s.scope.Name())
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return servers[0], nil
}

func statusAddresses(server *hcloud.Server) []clusterv1beta1.MachineAddress {
	// populate addresses
	addresses := []clusterv1beta1.MachineAddress{}

	if ip := server.PublicNet.IPv4.IP.String(); ip != "" {
		addresses = append(
			addresses,
			clusterv1beta1.MachineAddress{
				Type:    clusterv1beta1.MachineExternalIP,
				Address: ip,
			},
		)
	}

	if unicastIP := server.PublicNet.IPv6.IP; unicastIP.IsGlobalUnicast() {
		// Create a copy. This is important, otherwise we modify the IP of `server`. This could lead
		// to unexpected behaviour.
		ip := append(net.IP(nil), unicastIP...)

		// Hetzner returns the routed /64 base, increment last byte to obtain first usable address
		// The local value gets changed, not the IP of `server`.
		ip[15]++

		addresses = append(
			addresses,
			clusterv1beta1.MachineAddress{
				Type:    clusterv1beta1.MachineExternalIP,
				Address: ip.String(),
			},
		)
	}

	for _, net := range server.PrivateNet {
		addresses = append(
			addresses,
			clusterv1beta1.MachineAddress{
				Type:    clusterv1beta1.MachineInternalIP,
				Address: net.IP.String(),
			},
		)
	}

	return addresses
}

func (s *Service) createLabels() map[string]string {
	var machineType string
	if s.scope.IsControlPlane() {
		machineType = "control_plane"
	} else {
		machineType = "worker"
	}

	return map[string]string{
		infrav1.NameHetznerProviderOwned + s.scope.HetznerCluster.Name: string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:                                      s.scope.Name(),
		"machine_type":                                                 machineType,
	}
}

func updateHCloudMachineStatusFromServer(hm *infrav1.HCloudMachine, server *hcloud.Server) {
	hm.Status.Addresses = statusAddresses(server)
	hm.Status.InstanceState = ptr.To(server.Status)
}

// getSSHPrivateKey retrieves the SSH private key used for connecting to the rescue systems.
// It reads the key from the Kubernetes secret referenced by HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.
// On failure it sets SSHPrivateKeyAvailableCondition with a specific reason describing the root cause.
func (s *Service) getSSHPrivateKey(ctx context.Context) (string, error) {
	robotSecretName := s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name
	if robotSecretName == "" {
		v1beta1conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.SSHPrivateKeyAvailableCondition,
			infrav1.SSHPrivateKeySecretRefNotConfiguredReason,
			clusterv1beta1.ConditionSeverityError,
			"HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name is empty",
		)
		return "", fmt.Errorf("%w: HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name is empty. Can not get ssh client", errSSHKeyMisconfigured)
	}

	secretManager := secretutil.NewSecretManager(s.scope.Logger, s.scope.Client, s.scope.APIReader)

	robotSecret, err := secretManager.ObtainSecret(ctx, types.NamespacedName{
		Name:      robotSecretName,
		Namespace: s.scope.Namespace(),
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			v1beta1conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.SSHPrivateKeyAvailableCondition,
				infrav1.SSHPrivateKeySecretNotFoundReason,
				clusterv1beta1.ConditionSeverityWarning,
				"secret %s/%s not found", s.scope.Namespace(), robotSecretName,
			)
		}

		return "", fmt.Errorf("failed to get secret %q: %w", robotSecretName, err)
	}

	privateKey := string(robotSecret.Data[s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Key.PrivateKey])
	if privateKey == "" {
		v1beta1conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.SSHPrivateKeyAvailableCondition,
			infrav1.SSHPrivateKeyFieldEmptyReason,
			clusterv1beta1.ConditionSeverityError,
			"key %q in secret %q is missing or empty",
			s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Key.PrivateKey,
			robotSecretName,
		)
		return "", fmt.Errorf("key %q in secret %q is missing or empty. Failed to get ssh-private-key",
			s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Key.PrivateKey,
			robotSecretName)
	}

	return privateKey, nil
}

// getSSHClient uses HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef to get the ssh private key.
// Then it creates a sshClient connected to the first IP of the HCloudMachine.
func (s *Service) getSSHClient(ctx context.Context) (sshclient.Client, error) {
	hm := s.scope.HCloudMachine

	// retrieve the SSH private key from the secret referenced by HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.
	privateKey, err := s.getSSHPrivateKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("getSSHPrivateKey failed: %w", err)
	}

	if len(hm.Status.Addresses) == 0 {
		// This should never happen.
		return nil, errors.New("internal error: HCloudMachine.Status.Addresses empty. Can not connect via ssh")
	}
	ip := hm.Status.Addresses[0].Address

	// Unfortunately the hcloud API does not provide the sshd hostkey of the rescue system.
	// We need to trust the network. In theory a man-in-the-middle attack is possible.
	hcloudSSHClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		IP:         ip,
		PrivateKey: privateKey,
		Port:       22,
	})
	return hcloudSSHClient, nil
}
