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
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors" //nolint:staticcheck // we will handle that, when we update to capi v1.11
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	secretutil "github.com/syself/cluster-api-provider-hetzner/pkg/secrets"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	hcloudutil "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/util"
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

var errServerCreateNotPossible = fmt.Errorf("server create not possible - need action")

// Service defines struct with machine scope to reconcile HCloudMachines.
type Service struct {
	scope *scope.MachineScope
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
	conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedInstanceReadyCondition)
	conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedInstanceBootstrapReadyCondition)
	conditions.Delete(s.scope.HCloudMachine, infrav1.DeprecatedRateLimitExceededCondition)

	// detect failure domain
	failureDomain, err := s.scope.GetFailureDomain()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get failure domain: %w", err)
	}

	// set region in status of machine
	s.scope.SetRegion(failureDomain)

	// waiting for bootstrap data to be ready
	if !s.scope.IsBootstrapDataReady() {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.BootstrapReadyCondition,
			infrav1.BootstrapNotReadyReason,
			clusterv1.ConditionSeverityInfo,
			"bootstrap not ready yet",
		)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	conditions.MarkTrue(s.scope.HCloudMachine, infrav1.BootstrapReadyCondition)

	var server *hcloud.Server
	if s.scope.HCloudMachine.Spec.ProviderID == nil && s.scope.HCloudMachine.Spec.ImageURL != "" {
		// This is a new machine with imageURL.
		// Do some pre-flight checks, so we do not waste the hcloud-api budget if something is wrong
		if s.scope.ImageURLCommand == "" {
			msg := "imageURL is set, but the caph command is missing the --hcloud-image-url-command"
			s.scope.Logger.Error(nil, msg)
			conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
				string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", msg)
			return reconcile.Result{RequeueAfter: 3 * time.Minute}, nil
		}

		_, hcloudSSHKeys, err := s.getSSHKeys(ctx)
		if err != nil {
			err = fmt.Errorf("getSSHKeys for hcloud imageURL provisioning failed: %w", err)
			s.scope.Logger.Error(err, "")
			conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
				string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", err.Error())
			return reconcile.Result{RequeueAfter: 3 * time.Minute}, nil
		}

		if len(hcloudSSHKeys) == 0 {
			msg := "no ssh keys found hcloud imageURL provisioning found"
			s.scope.Logger.Error(nil, msg)
			conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
				string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", msg)
			return reconcile.Result{RequeueAfter: 3 * time.Minute}, nil
		}

		// pre-flight checks passed.
	}

	if s.scope.HCloudMachine.Spec.ProviderID != nil {
		server, err = s.findServer(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("findServer: %w", err)
		}

		// findServer will return both server and error as nil, if the server was not found.
		if server == nil {
			// The server did disappear in HCloud? Maybe it was delete via web-UI.
			// We set MachineError. CAPI will delete machine.
			msg := fmt.Sprintf("hcloud server (%q) no longer available. Setting MachineError.",
				*s.scope.HCloudMachine.Spec.ProviderID)

			s.scope.Logger.Error(errors.New(msg), msg,
				"ProviderID", *s.scope.HCloudMachine.Spec.ProviderID,
				"BootState", s.scope.HCloudMachine.Status.BootState,
				"BootStateSince", s.scope.HCloudMachine.Status.BootStateSince,
			)

			s.scope.SetError(msg, capierrors.CreateMachineError)
			s.scope.HCloudMachine.SetBootState(infrav1.HCloudBootStateUnset)
			record.Warn(s.scope.HCloudMachine, "NoHCloudServerFound", msg)
			conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
				string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", msg)
			// no need to requeue.
			return reconcile.Result{}, nil
		}
	}

	if s.scope.HCloudMachine.Status.BootState != infrav1.HCloudBootStateUnset && s.scope.HCloudMachine.Spec.ProviderID == nil {
		msg := fmt.Sprintf("internal error BootState=%q and ProviderID nil. Deleting machine",
			s.scope.HCloudMachine.Status.BootState)
		s.scope.Logger.Error(nil, msg)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		s.scope.HCloudMachine.SetBootState(infrav1.HCloudBootStateUnset)
		record.Warn(s.scope.HCloudMachine, "NoHCloudServerFound", msg)
		conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
			string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}

	switch s.scope.HCloudMachine.Status.BootState {
	case infrav1.HCloudBootStateUnset:
		return s.handleBootStateUnset(ctx)
	case infrav1.HCloudBootStateInitializing:
		return s.handleBootStateInitializing(ctx, server)
	case infrav1.HCloudBootStateEnablingRescue:
		return s.handleBootStateEnablingRescue(ctx, server)
	case infrav1.HCloudBootStateBootingToRescue:
		return s.handleBootStateBootingToRescue(ctx, server)
	case infrav1.HCloudBootStateRunningImageCommand:
		return s.handleBootStateRunningImageCommand(ctx, server)
	case infrav1.HCloudBootStateWaitingForReboot:
		return s.handleBootStateWaitingForReboot(ctx, server)
	case infrav1.HCloudBootStateBootToRealOS:
		return s.handleBootToRealOS(ctx, server)
	case infrav1.HCloudBootStateOperatingSystemRunning:
		return s.handleOperatingSystemRunning(ctx, server)
	default:
		return reconcile.Result{}, fmt.Errorf("unknown BootState: %s", s.scope.HCloudMachine.Status.BootState)
	}
}

func (s *Service) handleBootStateUnset(ctx context.Context) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine

	if hm.Spec.ProviderID != nil && *hm.Spec.ProviderID != "" && hm.Spec.ImageURL == "" {
		// This machine seems to be an existing machine which was created before introducing
		// Status.BootState.

		var msg string
		if !hm.Status.Ready {
			hm.SetBootState(infrav1.HCloudBootStateBootToRealOS)
		} else {
			hm.SetBootState(infrav1.HCloudBootStateOperatingSystemRunning)
		}
		msg = fmt.Sprintf("Updating old resource (pre BootState) %s", hm.Status.BootState)

		s.scope.Logger.Info(msg)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"%s", msg)
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil
	}

	// if provider id is not set create the server.
	server, image, err := s.createServerFromImageNameOrURL(ctx)
	if err != nil {
		if errors.Is(err, errServerCreateNotPossible) {
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
				"%s", err.Error())
			return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to create server: %w", err)
	}

	updateHCloudMachineStatusFromServer(hm, server)

	s.scope.SetProviderID(server.ID)

	// Provisioning from a hcloud image like ubuntu-YY.MM takes roughly 11 seconds.
	// Provisioning from a snapshot takes roughly 140 seconds.
	// We do not want to do too many api-calls (rate-limiting). So we differentiate
	// between both cases.
	// These values get only used **once** after the server got created.

	requeueAfter := 140 * time.Second
	if image.RapidDeploy {
		requeueAfter = 10 * time.Second
	}
	conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
		string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
		"ProviderID set")
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (s *Service) handleBootStateInitializing(ctx context.Context, server *hcloud.Server) (res reconcile.Result, reterr error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	// analyze status of server
	switch server.Status {
	case hcloud.ServerStatusStarting, hcloud.ServerStatusInitializing:
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"hcloud server is %q", server.Status)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	case hcloud.ServerStatusRunning:
		// execute below code
	default:
		// some temporary status
		s.scope.Logger.Info("Unknown hcloud server status", "status", server.Status)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo, "hcloud server has unknown status: %q", server.Status)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Server is Running.

	_, hcloudSSHKeys, err := s.getSSHKeys(ctx)
	if err != nil {
		return res, fmt.Errorf("getSSHKeys failed: %w", err)
	}

	rescueOpts := &hcloud.ServerEnableRescueOpts{
		Type:    hcloud.ServerRescueTypeLinux64,
		SSHKeys: hcloudSSHKeys,
	}
	result, err := s.scope.HCloudClient.EnableRescueSystem(ctx, server, rescueOpts)
	if err != nil {
		return res, fmt.Errorf("EnableRescueSystem failed: %w", err)
	}

	// The API of hetzner is async. We get an Action-ID as result. We need to wait until the action
	// is done. After that we can trigger the reboot, so that the machine boots into the rescue
	// system.
	hm.Status.ActionIDEnableRescueSystem = result.Action.ID

	hm.SetBootState(infrav1.HCloudBootStateEnablingRescue)

	conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
		string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
		"waiting for rescue system to be enabled")
	return reconcile.Result{RequeueAfter: 4 * time.Second}, nil
}

func (s *Service) handleBootStateEnablingRescue(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	if hm.Status.ActionIDEnableRescueSystem == 0 {
		msg := "handleBootStateEnablingRescue ActionIdEnableRescueSystem not set? Can not continue. Provisioning Failed"
		s.scope.Logger.Error(nil, msg)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}

	if hm.Status.ActionIDEnableRescueSystem != actionDone {
		action, err := s.scope.HCloudClient.GetAction(ctx, hm.Status.ActionIDEnableRescueSystem)
		if err != nil {
			err = fmt.Errorf("GetAction failed: %w", err)
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
				"%s", err.Error())
			return reconcile.Result{}, err
		}

		if action.Finished.IsZero() {
			// not finished yet.
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
				"Waiting until Action RescueEnabled is finished")
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}

		err = action.Error()
		if err != nil {
			err = fmt.Errorf("action %+v failed (wait for rescue enabled): %w", action, err)
			s.scope.Logger.Error(err, "")
			s.scope.SetError(err.Error(), capierrors.CreateMachineError)
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", err.Error())
			return reconcile.Result{}, nil
		}

		s.scope.Logger.Info("Action RescueEnabled is finished",
			"actionDuration", action.Finished.Sub(action.Started),
			"finishedSince", time.Since(action.Finished),
			"actionStatus", action.Status)

		hm.Status.ActionIDEnableRescueSystem = actionDone
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"Action RescueEnabled is finished")
		// When the reboot is triggered immediately after the action is finished,
		// then the reboot might get ignored.
		return reconcile.Result{RequeueAfter: 4 * time.Second}, nil
	}

	// Now we know that the rescue-system was enabled. Up to now the PreRescueOS is running. Next
	// step is to reboot the server into the rescue system.
	rebootAction, err := s.scope.HCloudClient.Reboot(ctx, server)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("reboot failed: %w", err)
	}
	s.scope.Logger.Info("Reboot started",
		"actionID", rebootAction.ID,
		"actionStatus", rebootAction.Status,
		"action", rebootAction)
	hm.Status.ActionIDRebootToRescue = rebootAction.ID
	hm.Status.RebootToRescueCount = 1

	hm.SetBootState(infrav1.HCloudBootStateBootingToRescue)
	conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
		string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
		"reboot to rescue started")
	return reconcile.Result{RequeueAfter: 55 * time.Second}, nil
}

func (s *Service) handleBootStateBootingToRescue(ctx context.Context, server *hcloud.Server) (reconcile.Result, error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	duration := time.Since(hm.Status.BootStateSince.Time)
	if duration > 4*time.Minute {
		// timeout. Something has failed.
		msg := fmt.Sprintf("reaching rescue system has timed out after %s. Deleting machine",
			duration.Round(time.Second).String())
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}

	if hm.Status.ActionIDRebootToRescue == 0 {
		msg := "handleBootStateBootingToRescue ActionIdRebootToRescue not set? Can not continue. Provisioning Failed"
		s.scope.Logger.Error(nil, msg)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}
	if hm.Status.ActionIDRebootToRescue != actionDone {
		action, err := s.scope.HCloudClient.GetAction(ctx, hm.Status.ActionIDRebootToRescue)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("GetAction failed: %w", err)
		}

		if action.Finished.IsZero() {
			// not finished yet.
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
				"Waiting until Action RebootToRescue is finished")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		err = action.Error()
		if err != nil {
			err = fmt.Errorf("action %+v failed (RebootToRescue): %w", action, err)
			s.scope.Logger.Error(err, "")
			s.scope.SetError(err.Error(), capierrors.CreateMachineError)
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", err.Error())
			return reconcile.Result{}, nil
		}

		s.scope.Logger.Info("Action RebootToRescue is finished",
			"actionDuration", action.Finished.Sub(action.Started),
			"finishedSince", time.Since(action.Finished),
			"actionStatus", action.Status)

		hm.Status.ActionIDRebootToRescue = actionDone
	}

	hcloudSSHClient, err := s.getSSHClient(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getSSHClient failed (waiting for rescue running): %w", err)
	}

	output := hcloudSSHClient.GetHostName()
	if output.Err != nil {
		conditions.MarkFalse(hm,
			infrav1.ServerCreateSucceededCondition,
			"GetHostnameFailed",
			clusterv1.ConditionSeverityInfo,
			"%s", output.String())
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", output.String())
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil //nolint:nilerr
	}

	conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)

	remoteHostName := output.String()
	if remoteHostName == s.scope.Name() {
		duration := time.Since(hm.Status.BootStateSince.Time)
		if duration < time.Second*10 {
			// Directly after RebootToRescue Action is done, we might still be able to connect
			// to the old system. Requeue.
			msg := fmt.Sprintf("Hostname is %q (not 'rescue' yet).", remoteHostName)
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
				"%s", msg)
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		// Reboot has failed. Work around buggy hcloud API.
		// Note: We could use ssh here to avoid API calls.
		rebootAction, err := s.scope.HCloudClient.Reboot(ctx, server)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("reboot failed: %w", err)
		}
		hm.Status.ActionIDRebootToRescue = rebootAction.ID
		hm.Status.RebootToRescueCount++
		hm.Status.BootStateSince = metav1.Now()
		msg := "Hostname not 'rescue'. Reboot started (again)"
		s.scope.Logger.Info(msg,
			"actionID", rebootAction.ID,
			"actionStatus", rebootAction.Status,
			"action", rebootAction,
			"rebootToRescueCount", hm.Status.RebootToRescueCount,
			"durationInBootState", duration)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"%s", msg)
		return reconcile.Result{RequeueAfter: 55 * time.Second}, nil
	}

	if remoteHostName != "rescue" {
		msg := fmt.Sprintf("Remote hostname (via ssh) of hcloud server is %q. Expected 'rescue'. Deleting hcloud machine", remoteHostName)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}

	// Now we know that we are inside a rescue system.
	// image-url-command has not started yet. Start it.

	data, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("hcloud GetRawBootstrapData failed: %w", err)
	}
	exitStatus, stdouStderr, err := hcloudSSHClient.StartImageURLCommand(ctx, s.scope.ImageURLCommand, hm.Spec.ImageURL, data, s.scope.Name())
	if err != nil {
		err := fmt.Errorf("StartImageURLCommand failed (retrying): %w", err)
		// This could be a temporary network error. Retry.
		s.scope.Logger.Error(err, "",
			"ImageURLCommand", s.scope.ImageURLCommand,
			"exitStatus", exitStatus,
			"stdouStderr", stdouStderr)
		return reconcile.Result{}, err
	}

	if exitStatus != 0 {
		msg := "StartImageURLCommand failed with non-zero exit status. Deleting machine"
		s.scope.Logger.Error(nil, msg,
			"ImageURLCommand", s.scope.ImageURLCommand,
			"exitStatus", exitStatus,
			"stdouStderr", stdouStderr)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}
	conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
		string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
		"hcloud-image-url-command started")
	hm.SetBootState(infrav1.HCloudBootStateRunningImageCommand)
	return reconcile.Result{RequeueAfter: 55 * time.Second}, nil
}

func (s *Service) handleBootStateRunningImageCommand(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	hcloudSSHClient, err := s.getSSHClient(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getSSHClient failed (wait for image-url-command): %w", err)
	}

	state, logFile, err := hcloudSSHClient.StateOfImageURLCommand()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("StateOfImageURLCommand failed: %w", err)
	}

	duration := time.Since(hm.Status.BootStateSince.Time)
	// Please keep the number (7) in sync with the docstring of ImageURL.
	if duration > 7*time.Minute {
		// timeout. Something has failed.
		msg := fmt.Sprintf("ImageURLCommand timed out after %s. Deleting machine",
			duration.Round(time.Second).String())
		err = errors.New(msg)
		s.scope.Logger.Error(err, "",
			"logFile", logFile)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}

	switch state {
	case sshclient.ImageURLCommandStateRunning:
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"hcloud-image-url-command still running")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil

	case sshclient.ImageURLCommandStateFinishedSuccessfully:
		rebootAction, err := s.scope.HCloudClient.Reboot(ctx, server)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("reboot after ImageURLCommand failed: %w",
				err)
		}
		hm.Status.ActionIDRebootAfterImageURLCommand = rebootAction.ID
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"hcloud-image-url-command finished successfully")
		hm.SetBootState(infrav1.HCloudBootStateWaitingForReboot)
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil

	case sshclient.ImageURLCommandStateFinishedFailed:
		msg := "ImageURLCommand failed. Deleting machine"
		err = errors.New(msg)
		s.scope.Logger.Error(err, "",
			"logFile", logFile)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	case sshclient.ImageURLCommandStateNotStarted:
		return reconcile.Result{}, fmt.Errorf("image-url-command not started in BootState %q? Should not happen",
			state)
	default:
		return reconcile.Result{}, fmt.Errorf("unknown ImageURLCommandState: %q", state)
	}
}

func (s *Service) handleBootStateWaitingForReboot(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	if hm.Status.ActionIDRebootAfterImageURLCommand == 0 {
		msg := "handleBootStateWaitingForReboot ActionIDRebootAfterImageURLCommand not set? Can not continue. Provisioning Failed"
		s.scope.Logger.Error(nil, msg)
		s.scope.SetError(msg, capierrors.CreateMachineError)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return reconcile.Result{}, nil
	}

	if hm.Status.ActionIDRebootAfterImageURLCommand != actionDone {
		action, err := s.scope.HCloudClient.GetAction(ctx, hm.Status.ActionIDRebootAfterImageURLCommand)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("GetAction failed: %w", err)
		}

		if action.Finished.IsZero() {
			// not finished yet.
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
				"Waiting until Action RebootAfterImageURLCommand is finished")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}

		err = action.Error()
		if err != nil {
			err = fmt.Errorf("action %+v failed (Reboot after hcloud-image-url-command): %w", action, err)
			s.scope.Logger.Error(err, "")
			msg := err.Error()
			s.scope.SetError(msg, capierrors.CreateMachineError)
			conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
				string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
				"%s", msg)
			return reconcile.Result{}, nil
		}

		s.scope.Logger.Info("Action RebootAfterImageURLCommand is finished",
			"actionDuration", action.Finished.Sub(action.Started),
			"finishedSince", time.Since(action.Finished),
			"actionStatus", action.Status)

		hm.Status.ActionIDRebootAfterImageURLCommand = actionDone
	}
	hm.SetBootState(infrav1.HCloudBootStateBootToRealOS)
	conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
		string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
		"reboot after hcloud-image-url-command finished. Starting real OS")
	return reconcile.Result{RequeueAfter: 55 * time.Second}, nil
}

func (s *Service) handleBootToRealOS(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	// analyze status of server
	switch server.Status {
	case hcloud.ServerStatusOff:
		return s.handleServerStatusOff(ctx, server)

	case hcloud.ServerStatusStarting, hcloud.ServerStatusInitializing:
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"hcloud server status: %s", server.Status)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil

	case hcloud.ServerStatusRunning:
		hm.SetBootState(infrav1.HCloudBootStateOperatingSystemRunning)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"hcloud server status: %s", server.Status)
		// Show changes in Status and go to next BootState.
		return reconcile.Result{RequeueAfter: requeueImmediately}, nil

	default:
		// some temporary status
		msg := fmt.Sprintf("hcloud server status unknown: %s", server.Status)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"%s", msg)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func (s *Service) handleOperatingSystemRunning(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	hm := s.scope.HCloudMachine
	updateHCloudMachineStatusFromServer(hm, server)

	// Clean up old Status fields
	hm.Status.ActionIDEnableRescueSystem = 0
	hm.Status.ActionIDRebootToRescue = 0
	hm.Status.ActionIDRebootAfterImageURLCommand = 0

	// check whether server is attached to the network
	if err := s.reconcileNetworkAttachment(ctx, server); err != nil {
		reterr := fmt.Errorf("failed to reconcile network attachment: %w", err)
		conditions.MarkFalse(
			hm,
			infrav1.ServerAvailableCondition,
			infrav1.NetworkAttachFailedReason,
			clusterv1.ConditionSeverityError,
			"%s",
			reterr.Error(),
		)
		return res, reterr
	}

	// nothing to do any more for worker nodes
	if !s.scope.IsControlPlane() {
		conditions.MarkTrue(hm, infrav1.ServerAvailableCondition)
		s.scope.SetReady(true)
		return res, nil
	}

	// all control planes have to be attached to the load balancer if it exists
	res, err = s.reconcileLoadBalancerAttachment(ctx, server)
	if err != nil {
		reterr := fmt.Errorf("failed to reconcile load balancer attachment: %w", err)
		conditions.MarkFalse(
			hm,
			infrav1.ServerAvailableCondition,
			infrav1.LoadBalancerAttachFailedReason,
			clusterv1.ConditionSeverityError,
			"%s",
			reterr.Error(),
		)
		return res, reterr
	}

	s.scope.SetReady(true)
	conditions.MarkTrue(hm, infrav1.ServerAvailableCondition)
	return reconcile.Result{}, nil
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
	hcloudutil.HandleRateLimitExceeded(hm, err, functionName)
	return fmt.Errorf("%s: %w", errMsg, err)
}

// Delete implements delete method of server.
func (s *Service) Delete(ctx context.Context) (res reconcile.Result, err error) {
	server, err := s.findServer(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to find server: %w", err)
	}

	// if no server has been found, then nothing can be deleted
	if server == nil {
		msg := fmt.Sprintf("Unable to delete HCloud server. Could not find matching server for %s", s.scope.Name())
		s.scope.V(1).Info(msg)
		record.Warnf(s.scope.HCloudMachine, "NoInstanceFound", msg)
		return res, nil
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

	// remove server from load balancer if it's being deleted
	if conditions.Has(s.scope.Machine, clusterv1.PreDrainDeleteHookSucceededCondition) {
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
	if hasPrivateIP && conditions.IsFalse(s.scope.HetznerCluster, infrav1.LoadBalancerReadyCondition) {
		return reconcile.Result{}, nil
	}

	// attach only if server has private IP or public IPv4, otherwise Hetzner cannot handle it
	if server.PublicNet.IPv4.IP == nil && !hasPrivateIP {
		return reconcile.Result{}, nil
	}

	apiServerPodHealthy := s.scope.Cluster.Spec.ControlPlaneRef == nil ||
		s.scope.Cluster.Spec.ControlPlaneRef.Kind != "KubeadmControlPlane" ||
		conditions.IsTrue(s.scope.Machine, controlplanev1.MachineAPIServerPodHealthyCondition)

	// we attach only nodes with kube-apiserver pod healthy to avoid downtime, skipped for the first node
	if len(s.scope.HetznerCluster.Status.ControlPlaneLoadBalancer.Target) > 0 && !apiServerPodHealthy {
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityInfo,
			"reconcile LoadBalancer: apiserver pod not healthy yet.")
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
	// Validate that ImageURLCommand is given
	hm := s.scope.HCloudMachine
	if s.scope.ImageURLCommand == "" {
		msg := fmt.Sprintf("controller was started without --hcloud-image-url-command. Provisioning via ImageURL %q not possible", hm.Spec.ImageURL)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return nil, nil, errors.New(msg)
	}

	image, err := s.getServerImage(ctx, preRescueOSImage)
	if err != nil {
		err = fmt.Errorf("failed to get pre-rescue-OS server image %q: %w", preRescueOSImage, err)
		msg := err.Error()
		record.Warn(hm, "FailedGetServerImage", msg)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return nil, nil, err
	}
	server, err := s.createServer(ctx, nil, image)
	if err != nil {
		return nil, nil, err
	}
	s.scope.HCloudMachine.SetBootState(infrav1.HCloudBootStateInitializing)
	return server, image, nil
}

func (s *Service) createServerFromImageName(ctx context.Context) (*hcloud.Server, *hcloud.Image, error) {
	hm := s.scope.HCloudMachine
	userData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get raw bootstrap data: %s", err)
		msg := err.Error()
		record.Warn(hm, "FailedGetBootstrapData", msg)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", err.Error())
		return nil, nil, err
	}

	image, err := s.getServerImage(ctx, hm.Spec.ImageName)
	if err != nil {
		err = fmt.Errorf("create server from imageName (%q): %w", hm.Spec.ImageName, err)
		msg := err.Error()
		record.Warn(hm, "FailedGetServerImage", msg)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return nil, nil, err
	}

	server, err := s.createServer(ctx, userData, image)
	if err != nil {
		return nil, nil, err
	}

	hm.SetBootState(infrav1.HCloudBootStateBootToRealOS)
	return server, image, nil
}

func (s *Service) createServer(ctx context.Context, userData []byte, image *hcloud.Image) (*hcloud.Server, error) {
	hm := s.scope.HCloudMachine
	automount := false
	startAfterCreate := true
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
			conditions.MarkFalse(hm,
				infrav1.ServerCreateSucceededCondition,
				infrav1.InstanceHasNonExistingPlacementGroupReason,
				clusterv1.ConditionSeverityError,
				"Placement group %q does not exist in cluster",
				*hm.Spec.PlacementGroupName,
			)
			return nil, errServerCreateNotPossible
		}
	}

	caphSSHKeys, hcloudSSHKeys, err := s.getSSHKeys(ctx)
	if err != nil {
		return nil, err
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
	server, err := s.scope.HCloudClient.CreateServer(ctx, opts)
	if err != nil {
		if hcloudutil.HandleRateLimitExceeded(hm, err, "CreateServer") {
			// RateLimit was reached. Condition and Event got already created.
			return nil, fmt.Errorf("failed to create HCloud server %s: %w", hm.Name, err)
		}
		// No condition was set yet. Set a general condition to false.
		conditions.MarkFalse(
			hm,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ServerCreateFailedReason,
			clusterv1.ConditionSeverityWarning,
			"%s",
			err.Error(),
		)
		record.Warnf(hm,
			"FailedCreateHCloudServer",
			"Failed to create HCloud server %s: %s",
			s.scope.Name(),
			err,
		)
		errMsg := fmt.Sprintf("failed to create HCloud server %s", hm.Name)
		return nil, handleRateLimit(hm, err, "CreateServer", errMsg)
	}

	// set ssh keys to status
	hm.Status.SSHKeys = caphSSHKeys

	conditions.MarkTrue(hm, infrav1.ServerCreateSucceededCondition)
	record.Eventf(hm, "SuccessfulCreate", "Created new server %s with ID %d", server.Name, server.ID)
	return server, nil
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
			conditions.MarkFalse(
				s.scope.HCloudMachine,
				infrav1.ServerCreateSucceededCondition,
				infrav1.SSHKeyNotFoundReason,
				clusterv1.ConditionSeverityError,
				"ssh key %q not present in hcloud",
				sshKeySpec.Name,
			)
			return nil, nil, errServerCreateNotPossible
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
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "GetServerType", "failed to get server type in HCloud")
	}
	if serverType == nil {
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ServerTypeNotFoundReason,
			clusterv1.ConditionSeverityError,
			"failed to get server type - nil type",
		)
		return nil, errServerCreateNotPossible
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
		err := fmt.Errorf("image is ambiguous - %d images have name %s",
			len(images), imageName)
		record.Warnf(s.scope.HCloudMachine, "ImageNameAmbiguous", err.Error())
		conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ImageAmbiguousReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, errServerCreateNotPossible
	}
	if len(images) == 0 {
		err := fmt.Errorf("no image found with name %s", s.scope.HCloudMachine.Spec.ImageName)
		record.Warnf(s.scope.HCloudMachine, "ImageNotFound", err.Error())
		conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerCreateSucceededCondition,
			infrav1.ImageNotFoundReason,
			clusterv1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		return nil, errServerCreateNotPossible
	}

	return images[0], nil
}

func (s *Service) handleServerStatusOff(ctx context.Context, server *hcloud.Server) (res reconcile.Result, err error) {
	// Check if server is in ServerStatusOff and turn it on. This is to avoid a bug of Hetzner where
	// sometimes machines are created and not turned on

	serverAvailableCondition := conditions.Get(s.scope.HCloudMachine, infrav1.ServerAvailableCondition)
	if serverAvailableCondition != nil &&
		serverAvailableCondition.Status == corev1.ConditionFalse &&
		serverAvailableCondition.Reason == infrav1.ServerOffReason {
		s.scope.Info("Trigger power on again")
		if time.Now().Before(serverAvailableCondition.LastTransitionTime.Time.Add(serverOffTimeout)) {
			// Not yet timed out, try again to power on
			if err := s.scope.HCloudClient.PowerOnServer(ctx, server); err != nil {
				if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
					// if server is locked, we just retry again
					conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
						string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityInfo,
						"handleServerStatusOff: server locked. Will retry")
					return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
				}
				return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "PowerOnServer", "failed to power on server")
			}
		} else {
			// Timed out. Set failure reason
			s.scope.SetError("reached timeout of waiting for machines that are switched off", capierrors.CreateMachineError)
			return res, nil
		}
	} else {
		// No condition set yet. Try to power server on.
		if err := s.scope.HCloudClient.PowerOnServer(ctx, server); err != nil {
			if hcloud.IsError(err, hcloud.ErrorCodeLocked) {
				// if server is locked, we just retry again
				conditions.MarkFalse(s.scope.HCloudMachine, infrav1.ServerAvailableCondition,
					string(s.scope.HCloudMachine.Status.BootState), clusterv1.ConditionSeverityInfo, "handleServerStatusOff: server locked. Will retry")
				return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return reconcile.Result{}, handleRateLimit(s.scope.HCloudMachine, err, "PowerOnServer", "failed to power on server")
		}
		conditions.MarkFalse(
			s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.ServerOffReason,
			clusterv1.ConditionSeverityInfo,
			"server is switched off",
		)
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

		conditions.MarkFalse(s.scope.HCloudMachine,
			infrav1.ServerAvailableCondition,
			infrav1.ServerTerminatingReason,
			clusterv1.ConditionSeverityInfo,
			"Instance has been shut down",
		)

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

func (s *Service) findServer(ctx context.Context) (*hcloud.Server, error) {
	var server *hcloud.Server

	// try to find the server based on its id
	serverID, err := s.scope.ServerIDFromProviderID()
	if err == nil {
		server, err = s.scope.HCloudClient.GetServer(ctx, serverID)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get server %d", serverID)
			return nil, handleRateLimit(s.scope.HCloudMachine, err, "GetServer", errMsg)
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
		return nil, handleRateLimit(s.scope.HCloudMachine, err, "ListServers", "failed to list servers")
	}

	if len(servers) > 1 {
		err := fmt.Errorf("found %d servers with name %s", len(servers), s.scope.Name())
		record.Warnf(s.scope.HCloudMachine, "MultipleInstances", err.Error())
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	s.scope.Logger.Info("DeprecationWarning finding Server by labels is no longer needed. We plan to remove that feature and rename findServer to getServer", "err", err)

	return servers[0], nil
}

func statusAddresses(server *hcloud.Server) []clusterv1.MachineAddress {
	// populate addresses
	addresses := []clusterv1.MachineAddress{}

	if ip := server.PublicNet.IPv4.IP.String(); ip != "" {
		addresses = append(
			addresses,
			clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
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
			clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: ip.String(),
			},
		)
	}

	for _, net := range server.PrivateNet {
		addresses = append(
			addresses,
			clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
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

// getSSHClient uses HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef to get the ssh private key.
// Then it creates a sshClient connected to the first IP of the HCloudMachine.
func (s *Service) getSSHClient(ctx context.Context) (sshclient.Client, error) {
	hm := s.scope.HCloudMachine

	robotSecretName := s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name
	if robotSecretName == "" {
		msg := "HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Name is empty. Can not set hcloud machine into rescue-mode without ssh private key"
		err := errors.New(msg)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return nil, err
	}

	secretManager := secretutil.NewSecretManager(s.scope.Logger, s.scope.Client, s.scope.APIReader)
	robotSecret, err := secretManager.ObtainSecret(ctx, types.NamespacedName{
		Name:      robotSecretName,
		Namespace: s.scope.Namespace(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %q: %w", robotSecretName, err)
	}
	if robotSecret == nil {
		return nil, fmt.Errorf("failed to obtain secret %s/%s", s.scope.Namespace(), robotSecretName)
	}

	if len(hm.Status.Addresses) == 0 {
		msg := "HCloudMachine.Status.Addresses empty. Can not connect via ssh"
		s.scope.Logger.Error(nil, msg)
		conditions.MarkFalse(hm, infrav1.ServerAvailableCondition,
			string(hm.Status.BootState), clusterv1.ConditionSeverityWarning,
			"%s", msg)
		return nil, errors.New(msg)
	}

	privateKey := string(robotSecret.Data[s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Key.PrivateKey])
	if privateKey == "" {
		return nil, fmt.Errorf("key %q in secret %q is missing or empty. Failed to get ssh-private-key",
			s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef.Key.PrivateKey,
			robotSecretName)
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
