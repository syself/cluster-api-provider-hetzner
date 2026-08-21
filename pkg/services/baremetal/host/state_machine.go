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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// hostStateMachine is a finite state machine that manages transitions between
// the states of a BareMetalHost.
type hostStateMachine struct {
	host       *infrav2.HetznerBareMetalHost
	reconciler *Service
	nextState  infrav2.ProvisioningState
	log        logr.Logger
}

var errNoHandlerFound = fmt.Errorf("no handler found")

func newHostStateMachine(host *infrav2.HetznerBareMetalHost, reconciler *Service, log logr.Logger) *hostStateMachine {
	currentState := host.Status.ProvisioningState
	r := hostStateMachine{
		host:       host,
		reconciler: reconciler,
		nextState:  currentState, // Remain in current state by default
		log:        log,
	}
	return &r
}

type stateHandler func(ctx context.Context) actionResult

func (hsm *hostStateMachine) handlers() map[infrav2.ProvisioningState]stateHandler {
	return map[infrav2.ProvisioningState]stateHandler{
		infrav2.StatePreparing:         hsm.handlePreparing,
		infrav2.StateRegistering:       hsm.handleRegistering,
		infrav2.StatePreProvisioning:   hsm.handlePreProvisioning,
		infrav2.StateImageInstalling:   hsm.handleImageInstalling,
		infrav2.StateEnsureProvisioned: hsm.handleEnsureProvisioned,
		infrav2.StateProvisioned:       hsm.handleProvisioned,
		infrav2.StateDeprovisioning:    hsm.handleDeprovisioning,
		infrav2.StateDeleting:          hsm.handleDeleting,
	}
}

func (hsm *hostStateMachine) ReconcileState(ctx context.Context) (actionRes actionResult) {
	initialState := hsm.host.Status.ProvisioningState
	defer func() {
		if hsm.nextState != initialState {
			hsm.log.V(1).Info("changing provisioning state", "old", initialState, "new", hsm.nextState)
			hsm.host.Status.ProvisioningState = hsm.nextState

			cond := conditions.Get(hsm.host, infrav2.HetznerBareMetalHostProvisionSucceededCondition)
			if cond != nil && cond.Reason == infrav2.HetznerBareMetalHostProvisioningReason {
				markProvisionPending(hsm.host, hsm.nextState)
			}
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
	deprecatedv1beta1conditions.MarkTrue(hsm.host, infrav2.CredentialsAvailableV1Beta1Condition)
	conditions.Set(hsm.host, metav1.Condition{
		Type:   infrav2.HetznerBareMetalHostSSHKeysAvailableCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav2.HetznerBareMetalHostSSHKeysAvailableReason,
	})

	// This state was removed. We have to handle the edge-case where
	// the controller got updated and a machine
	// is in this state. Installing the image again should solve that.
	if initialState == "provisioning" {
		hsm.log.Info("edge-case was hit: New code meets machine in removed state 'provisioning'. Re-setting",
			"new-state", infrav2.StateImageInstalling)
		initialState = infrav2.StateImageInstalling
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
		hsm.nextState = infrav2.StateDeleting
	case infrav2.StateRegistering, infrav2.StateImageInstalling,
		infrav2.StateEnsureProvisioned, infrav2.StateProvisioned:
		hsm.nextState = infrav2.StateDeprovisioning
	case infrav2.StateDeprovisioning:
		// Continue deprovisioning.
		return false
	}
	return true
}

func (hsm *hostStateMachine) updateSSHKey() actionResult {
	// Skip if deprovisioning
	if hsm.host.Status.ProvisioningState == infrav2.StateDeprovisioning {
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
	if hsm.host.Status.SSHStatus.CurrentOS == nil {
		if err := hsm.host.UpdateOSSSHStatus(*osSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update OS SSH secret status: %w", err)}
		}
	}

	if !hsm.host.Status.SSHStatus.CurrentOS.Match(*osSSHSecret) {
		// Take action depending on state
		switch hsm.nextState {
		case infrav2.StateEnsureProvisioned:
			// Go back to StateImageInstalling as we need to provision again
			hsm.nextState = infrav2.StateImageInstalling
		case infrav2.StateProvisioned:
			errMessage := "secret has been modified although a provisioned machine uses it"
			record.Event(hsm.host, "SSHSecretUnexpectedlyModified", errMessage)
			hsm.host.SetError(infrav2.RegistrationError, errMessage)
			// The user has to fix the secret. Check again in five minutes.
			return actionContinue{delay: 5 * time.Minute}
		}
		if err := hsm.host.UpdateOSSSHStatus(*osSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update status of OS SSH secret: %w", err)}
		}
	}
	if err := validateSSHKey(osSSHSecret, hsm.reconciler.scope.HetznerBareMetalMachine.Spec.SSHSpec.SecretRef); err != nil {
		msg := fmt.Sprintf("ssh credentials are invalid: %s", err.Error())
		deprecatedv1beta1conditions.MarkFalse(
			hsm.host,
			infrav2.CredentialsAvailableV1Beta1Condition,
			infrav2.SSHCredentialsInSecretInvalidV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			msg,
		)
		conditions.Set(hsm.host, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostSSHKeysAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostSSHKeysInvalidReason,
			Message: msg,
		})

		record.Warnf(hsm.host, infrav2.SSHKeyAlreadyExistsV1Beta1Reason, msg)
		hsm.host.SetError(infrav2.PreparationError, infrav2.ErrorMessageMissingOrInvalidSecretData)
		// The user has to fix the secret. Check again in five minutes.
		return actionContinue{delay: 5 * time.Minute}
	}
	return nil
}

func (hsm *hostStateMachine) updateRescueSSHStatusAndValidateKey(rescueSSHSecret *corev1.Secret) actionResult {
	// if status is not set yet, then update it
	if hsm.host.Status.SSHStatus.CurrentRescue == nil {
		if err := hsm.host.UpdateRescueSSHStatus(*rescueSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update rescue SSH secret status: %w", err)}
		}
	}

	if !hsm.host.Status.SSHStatus.CurrentRescue.Match(*rescueSSHSecret) {
		// Take action depending on state
		switch hsm.nextState {
		case infrav2.StatePreparing, infrav2.StateRegistering, infrav2.StateImageInstalling:
			msg := "stopped provisioning host as rescue ssh secret was updated"
			record.Warn(hsm.host, "HostProvisioningStopped", msg)
			hsm.log.V(1).Info(msg, "state", hsm.nextState)
			hsm.nextState = infrav2.StateNone
		}
		if err := hsm.host.UpdateRescueSSHStatus(*rescueSSHSecret); err != nil {
			return actionError{err: fmt.Errorf("failed to update status of rescue SSH secret: %w", err)}
		}
	}
	if err := validateSSHKey(rescueSSHSecret, hsm.reconciler.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef); err != nil {
		msg := fmt.Sprintf("ssh credentials for rescue system are invalid: %s", err.Error())
		deprecatedv1beta1conditions.MarkFalse(
			hsm.host,
			infrav2.CredentialsAvailableV1Beta1Condition,
			infrav2.SSHCredentialsInSecretInvalidV1Beta1Reason,
			clusterv1.ConditionSeverityError,
			"%s",
			msg,
		)
		conditions.Set(hsm.host, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostSSHKeysAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostSSHKeysInvalidReason,
			Message: msg,
		})
		hsm.host.SetError(infrav2.PreparationError, infrav2.ErrorMessageMissingOrInvalidSecretData)
		// The user has to fix the secret. Check again in five minutes.
		return actionContinue{delay: 5 * time.Minute}
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
		hsm.nextState = infrav2.StateDeprovisioning
		return actionComplete{}
	}

	record.Eventf(hsm.host, "PreparingForProvisioning", "ServerID %d %s", hsm.host.Spec.ServerID, hsm.host.Spec.Description)

	actResult := hsm.reconciler.actionPreparing(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav2.StateRegistering
	}
	return actResult
}

func (hsm *hostStateMachine) handleRegistering(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav2.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionRegistering(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav2.StatePreProvisioning
	}
	return actResult
}

func (hsm *hostStateMachine) handlePreProvisioning(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav2.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionPreProvisioning(ctx)
	switch actResult.(type) {
	case actionComplete:
		hsm.nextState = infrav2.StateImageInstalling
	case actionError:
		// re-enable rescue system. If actionPreProvisioning
		// failed with actionError, then it is likely that
		// the next run (without reboot) fails with this error:
		// ERROR unmounting device(s):
		// umount: /: target is busy.
		// Cannot continue, device(s) seem to be in use.
		// Please unmount used devices manually or reboot the rescue system and retry.
		hsm.nextState = infrav2.StatePreparing
	}

	return actResult
}

func (hsm *hostStateMachine) handleImageInstalling(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav2.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionImageInstalling(ctx)
	switch actResult.(type) {
	case actionComplete:
		hsm.nextState = infrav2.StateEnsureProvisioned
	case actionError:
		// re-enable rescue system. If installimage failed, then it is likely, that
		// the next run (without reboot) fails with this error:
		// ERROR unmounting device(s):
		// umount: /: target is busy.
		// Cannot continue, device(s) seem to be in use.
		// Please unmount used devices manually or reboot the rescue system and retry.
		hsm.nextState = infrav2.StatePreparing
	}

	return actResult
}

func (hsm *hostStateMachine) handleEnsureProvisioned(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav2.StateDeprovisioning
		return actionComplete{}
	}

	actResult := hsm.reconciler.actionEnsureProvisioned(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav2.StateProvisioned
	}
	return actResult
}

func (hsm *hostStateMachine) handleProvisioned(ctx context.Context) actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav2.StateDeprovisioning
		return actionComplete{}
	}
	return hsm.reconciler.actionProvisioned(ctx)
}

func (hsm *hostStateMachine) handleDeprovisioning(ctx context.Context) actionResult {
	actResult := hsm.reconciler.actionDeprovisioning(ctx)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav2.StateNone
		return actionComplete{}
	}
	return actResult
}

func (hsm *hostStateMachine) handleDeleting(ctx context.Context) actionResult {
	return hsm.reconciler.actionDeleting(ctx)
}

// provisioningCancelled returns true when the consuming machine or its owner CAPI Machine is gone
// or being deleted. The host then deprovisions. The machine used to signal this by clearing the
// installImage it had copied onto the host.
//
// The owner CAPI Machine is checked too: it can enter deletion before the HetznerBareMetalMachine
// gets its deletion timestamp, and it can be force deleted while the HetznerBareMetalMachine
// lingers. In both cases the host must deprovision instead of continuing to provision.
func (hsm *hostStateMachine) provisioningCancelled() bool {
	hbmm := hsm.reconciler.scope.HetznerBareMetalMachine
	if hbmm == nil || !hbmm.DeletionTimestamp.IsZero() {
		return true
	}
	machine := hsm.reconciler.scope.Machine
	return machine == nil || !machine.DeletionTimestamp.IsZero()
}
