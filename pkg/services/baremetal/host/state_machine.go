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

package host

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// hostStateMachine is a finite state machine that manages transitions between
// the states of a BareMetalHost.
type hostStateMachine struct {
	host       *infrav1.HetznerBareMetalHost
	reconciler *Service
	nextState  infrav1.ProvisioningState
	log        logr.Logger
}

var errNoHandlerFound = fmt.Errorf("no handler found")

func newHostStateMachine(host *infrav1.HetznerBareMetalHost, reconciler *Service, log logr.Logger) *hostStateMachine {
	currentState := host.Spec.Status.ProvisioningState
	r := hostStateMachine{
		host:       host,
		reconciler: reconciler,
		nextState:  currentState, // Remain in current state by default
		log:        log,
	}
	return &r
}

type stateHandler func(ctx context.Context) actionResult

func (hsm *hostStateMachine) handlers() map[infrav1.ProvisioningState]stateHandler {
	return map[infrav1.ProvisioningState]stateHandler{
		infrav1.StatePreparing:         hsm.handlePreparing,
		infrav1.StateRegistering:       hsm.handleRegistering,
		infrav1.StateImageInstalling:   hsm.handleImageInstalling,
		infrav1.StateEnsureProvisioned: hsm.handleEnsureProvisioned,
		infrav1.StateProvisioned:       hsm.handleProvisioned,
		infrav1.StateDeprovisioning:    hsm.handleDeprovisioning,
		infrav1.StateDeleting:          hsm.handleDeleting,
	}
}

func (hsm *hostStateMachine) ReconcileState(ctx context.Context) (actionRes actionResult) {
	initialState := hsm.host.Spec.Status.ProvisioningState
	defer func() {
		if hsm.nextState != initialState {
			hsm.log.V(1).Info("changing provisioning state", "old", initialState, "new", hsm.nextState)
			hsm.host.Spec.Status.ProvisioningState = hsm.nextState
		}
	}()

	if hsm.checkInitiateDelete() {
		return actionComplete{}
	}

	actResult := hsm.updateSSHKey()
	if _, complete := actResult.(actionComplete); !complete {
		return actResult
	}

	// Assume credentials are ready for now. This can be changed while the state is handled.
	conditions.MarkTrue(hsm.host, infrav1.CredentialsAvailableCondition)

	// This state was removed. We have to handle the edge-case where
	// the controller got updated and a machine
	// is in this state. Installing the image again should solve that.
	if initialState == "provisioning" {
		hsm.log.Info("edge-case was hit: New code meets machine in removed state 'provisioning'. Re-setting",
			"new-state", infrav1.StateImageInstalling)
		initialState = infrav1.StateImageInstalling
	}

	if stateHandler, found := hsm.handlers()[initialState]; found {
		return stateHandler(ctx)
	}

	return actionError{fmt.Errorf("%w: state %q", errNoHandlerFound, initialState)}
}

func (hsm *hostStateMachine) checkInitiateDelete() bool {
	if hsm.host.DeletionTimestamp.IsZero() {
		// Delete not requested
		return false
	}

	switch hsm.nextState {
	default:
		hsm.nextState = infrav1.StateDeleting
	case infrav1.StateRegistering, infrav1.StateImageInstalling,
		infrav1.StateEnsureProvisioned, infrav1.StateProvisioned:
		hsm.nextState = infrav1.StateDeprovisioning
	case infrav1.StateDeprovisioning:
		// Continue deprovisioning.
		return false
	}
	return true
}

func (hsm *hostStateMachine) updateSSHKey() actionResult {
	// Skip if deprovisioning
	if hsm.host.Spec.Status.ProvisioningState == infrav1.StateDeprovisioning {
		return actionComplete{}
	}

	osSSHSecret := hsm.reconciler.scope.OSSSHSecret
	rescueSSHSecret := hsm.reconciler.scope.RescueSSHSecret

	// Check whether os secret has been updated if it exists already
	if osSSHSecret != nil {
		if actResult := hsm.updateOSSSHStatusAndValidateKey(osSSHSecret); actResult != nil {
			return actResult
		}
	}

	// Check whether rescue secret has been updated if it exists already
	if rescueSSHSecret != nil {
		if actResult := hsm.updateRescueSSHStatusAndValidateKey(rescueSSHSecret); actResult != nil {
			return actResult
		}
	}
	return actionComplete{}
}

func (hsm *hostStateMachine) updateOSSSHStatusAndValidateKey(osSSHSecret *corev1.Secret) actionResult {
	// if status is not set yet, then update it
	if hsm.host.Spec.Status.SSHStatus.CurrentOS == nil {
		if err := hsm.host.UpdateOSSSHStatus(*osSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update OS SSH secret status: %w", err)}
		}
	}

	if !hsm.host.Spec.Status.SSHStatus.CurrentOS.Match(*osSSHSecret) {
		// Take action depending on state
		switch hsm.nextState {
		case infrav1.StateEnsureProvisioned:
			// Go back to StateImageInstalling as we need to provision again
			hsm.nextState = infrav1.StateImageInstalling
		case infrav1.StateProvisioned:
			errMessage := "secret has been modified although a provisioned machine uses it"
			record.Event(hsm.host, "SSHSecretUnexpectedlyModified", errMessage)
			return hsm.reconciler.recordActionFailure(infrav1.RegistrationError, errMessage)
		}
		if err := hsm.host.UpdateOSSSHStatus(*osSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update status of OS SSH secret: %w", err)}
		}
	}
	if err := validateSSHKey(osSSHSecret, hsm.host.Spec.Status.SSHSpec.SecretRef); err != nil {
		msg := fmt.Sprintf("ssh credentials are invalid: %s", err.Error())
		conditions.MarkFalse(
			hsm.host,
			infrav1.CredentialsAvailableCondition,
			infrav1.SSHCredentialsInSecretInvalidReason,
			clusterv1.ConditionSeverityError,
			"%s",
			msg,
		)

		record.Warnf(hsm.host, infrav1.SSHKeyAlreadyExistsReason, msg)
		return hsm.reconciler.recordActionFailure(infrav1.PreparationError, infrav1.ErrorMessageMissingOrInvalidSecretData)
	}
	return nil
}

func (hsm *hostStateMachine) updateRescueSSHStatusAndValidateKey(rescueSSHSecret *corev1.Secret) actionResult {
	// if status is not set yet, then update it
	if hsm.host.Spec.Status.SSHStatus.CurrentRescue == nil {
		if err := hsm.host.UpdateRescueSSHStatus(*rescueSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update rescue SSH secret status: %w", err)}
		}
	}

	if !hsm.host.Spec.Status.SSHStatus.CurrentRescue.Match(*rescueSSHSecret) {
		// Take action depending on state
		switch hsm.nextState {
		case infrav1.StatePreparing, infrav1.StateRegistering, infrav1.StateImageInstalling:
			msg := "stopped provisioning host as rescue ssh secret was updated"
			record.Warn(hsm.host, "HostProvisioningStopped", msg)
			hsm.log.V(1).Info(msg, "state", hsm.nextState)
			hsm.nextState = infrav1.StateNone
		}
		if err := hsm.host.UpdateRescueSSHStatus(*rescueSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update status of rescue SSH secret: %w", err)}
		}
	}
	if err := validateSSHKey(rescueSSHSecret, hsm.reconciler.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef); err != nil {
		msg := fmt.Sprintf("ssh credentials for rescue system are invalid: %s", err.Error())
		conditions.MarkFalse(
			hsm.host,
			infrav1.CredentialsAvailableCondition,
			infrav1.SSHCredentialsInSecretInvalidReason,
			clusterv1.ConditionSeverityError,
			"%s",
			msg,
		)
		return hsm.reconciler.recordActionFailure(infrav1.PreparationError, infrav1.ErrorMessageMissingOrInvalidSecretData)
	}
	return nil
}

func validateSSHKey(sshSecret *corev1.Secret, secretRef infrav1.SSHSecretRef) error {
	creds := sshclient.Credentials{
		Name:       string(sshSecret.Data[secretRef.Key.Name]),
		PublicKey:  string(sshSecret.Data[secretRef.Key.PublicKey]),
		PrivateKey: string(sshSecret.Data[secretRef.Key.PrivateKey]),
	}

	// Validate token
	if err := creds.Validate(); err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	return nil
}

func (hsm *hostStateMachine) handlePreparing(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	record.Eventf(hsm.host, "PreparingForProvisioning", "ServerID %d %s", hsm.host.Spec.ServerID, hsm.host.Spec.Description)

	actResult := hsm.reconciler.actionPreparing(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateRegistering
	}
	return actResult
}

func (hsm *hostStateMachine) handleRegistering(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionRegistering(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateImageInstalling
	}
	return actResult
}

func (hsm *hostStateMachine) handleImageInstalling(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionImageInstalling(ctx)
	switch actResult.(type) {
	case actionComplete:
		hsm.nextState = infrav1.StateEnsureProvisioned
	case actionError:
		// re-enable rescue system. If installimage failed, then it is likely, that
		// the next run (without reboot) fails with this error:
		// ERROR unmounting device(s):
		// umount: /: target is busy.
		// Cannot continue, device(s) seem to be in use.
		// Please unmount used devices manually or reboot the rescuesystem and retry.
		hsm.nextState = infrav1.StatePreparing
	}

	return actResult
}

func (hsm *hostStateMachine) handleEnsureProvisioned(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionEnsureProvisioned(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateProvisioned
	}
	return actResult
}

func (hsm *hostStateMachine) handleProvisioned(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}
	return hsm.reconciler.actionProvisioned(ctx)
}

func (hsm *hostStateMachine) handleDeprovisioning(ctx context.Context) actionResult {
	actResult := hsm.reconciler.actionDeprovisioning(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateNone
		return actionComplete{}
	}
	return actResult
}

func (hsm *hostStateMachine) handleDeleting(ctx context.Context) actionResult {
	return hsm.reconciler.actionDeleting(ctx)
}

func (hsm *hostStateMachine) provisioningCancelled() bool {
	return hsm.host.Spec.Status.InstallImage == nil
}
