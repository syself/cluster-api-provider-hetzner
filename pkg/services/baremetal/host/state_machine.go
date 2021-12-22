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
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
)

// hostStateMachine is a finite state machine that manages transitions between
// the states of a BareMetalHost.
type hostStateMachine struct {
	host       *infrav1.HetznerBareMetalHost
	reconciler *Service
	nextState  infrav1.ProvisioningState
	log        *logr.Logger
}

func newHostStateMachine(host *infrav1.HetznerBareMetalHost, reconciler *Service, log *logr.Logger) *hostStateMachine {
	currentState := host.Spec.Status.ProvisioningState
	r := hostStateMachine{
		host:       host,
		reconciler: reconciler,
		nextState:  currentState, // Remain in current state by default
		log:        log,
	}
	return &r
}

type stateHandler func() actionResult

func (hsm *hostStateMachine) handlers() map[infrav1.ProvisioningState]stateHandler {
	return map[infrav1.ProvisioningState]stateHandler{
		infrav1.StateNone:              hsm.handleNone,
		infrav1.StateRegistering:       hsm.handleRegistering,
		infrav1.StateAvailable:         hsm.handleAvailable,
		infrav1.StateImageInstalling:   hsm.handleImageInstalling,
		infrav1.StateProvisioning:      hsm.handleProvisioning,
		infrav1.StateEnsureProvisioned: hsm.handleEnsureProvisioned,
		infrav1.StateProvisioned:       hsm.handleProvisioned,
		infrav1.StateDeprovisioning:    hsm.handleDeprovisioning,
	}
}

func (hsm *hostStateMachine) ReconcileState(ctx context.Context) (actionRes actionResult) {
	initialState := hsm.host.Spec.Status.ProvisioningState
	defer func() {
		if hsm.nextState != initialState {
			hsm.log.Info("changing provisioning state", "old", initialState, "new", hsm.nextState)
			hsm.host.Spec.Status.ProvisioningState = hsm.nextState
		}
	}()

	actResult := hsm.updateSSHKey()
	if _, complete := actResult.(actionComplete); !complete {
		return actResult
	}

	if stateHandler, found := hsm.handlers()[initialState]; found {
		return stateHandler()
	}

	hsm.log.Info("No handler found for state", "state", initialState)
	return actionError{fmt.Errorf("no handler found for state \"%s\"", initialState)}
}

func (hsm *hostStateMachine) updateSSHKey() actionResult {
	// Get ssh key secrets from secret
	osSSHSecret, rescueSSHSecret := hsm.reconciler.getSSHKeysAndUpdateStatus()

	// Check whether os secret has been updated if it exists already
	if osSSHSecret != nil {
		if !hsm.host.Spec.Status.SSHStatus.CurrentOS.Match(*osSSHSecret) {
			// Take action depending on state
			switch hsm.nextState {
			case infrav1.StateProvisioning, infrav1.StateEnsureProvisioned:
				// Go back to StateImageInstalling as we need to provision again
				hsm.nextState = infrav1.StateImageInstalling
			case infrav1.StateProvisioned:
				errMessage := "secret has been modified although a provisioned machine uses it"
				record.Event(hsm.host, "SSHSecretUnexpectedlyModified", errMessage)
				return hsm.reconciler.recordActionFailure(infrav1.RegistrationError, errMessage)
			}
			hsm.host.UpdateOSSSHStatus(*osSSHSecret)
		}
		actResult := hsm.reconciler.validateSSHKey(osSSHSecret, "os")
		if _, complete := actResult.(actionComplete); !complete {
			return actResult
		}
	}

	if !hsm.host.Spec.Status.SSHStatus.CurrentRescue.Match(*rescueSSHSecret) {
		// Take action depending on state
		switch hsm.nextState {
		case infrav1.StateRegistering, infrav1.StateAvailable, infrav1.StateImageInstalling:
			hsm.log.Info("Attention: Going back to state none as rescue secret was updated", "state", hsm.nextState,
				"currentRescue", hsm.host.Spec.Status.SSHStatus.CurrentRescue, "newRescue", rescueSSHSecret)
			hsm.nextState = infrav1.StateNone
		case infrav1.StateDeprovisioning:
			// Remove all possible information of the bare metal machine from host and then go to StateNone
			hsm.reconciler.actionDeprovisioning()
			hsm.log.Info("Attention: Going back to state none as rescue secret was updated", "state", hsm.nextState,
				"currentRescue", hsm.host.Spec.Status.SSHStatus.CurrentRescue, "newRescue", rescueSSHSecret)
			hsm.nextState = infrav1.StateNone
		}
		hsm.host.UpdateRescueSSHStatus(*rescueSSHSecret)
	}

	return hsm.reconciler.validateSSHKey(rescueSSHSecret, "rescue")
}

func (hsm *hostStateMachine) handleNone() actionResult {
	actResult := hsm.reconciler.actionNone()
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateRegistering
	}
	return actResult
}

func (hsm *hostStateMachine) handleRegistering() actionResult {
	actResult := hsm.reconciler.actionEnsureCorrectBoot(rescue, nil)
	if _, ok := actResult.(actionComplete); ok {
		actResult := hsm.reconciler.actionRegistering()
		if _, ok := actResult.(actionComplete); ok {
			hsm.nextState = infrav1.StateAvailable
		}
	}
	return actResult
}

func (hsm *hostStateMachine) handleAvailable() actionResult {
	actResult := hsm.reconciler.actionAvailable()
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateImageInstalling
	}
	return actResult
}

func (hsm *hostStateMachine) handleImageInstalling() actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	// If the ssh reboot can be used, then only in the case where the server has been deprovisioned before
	// and still uses the port after cloud init.
	port := hsm.host.Spec.Status.SSHSpec.PortAfterCloudInit
	actResult := hsm.reconciler.actionEnsureCorrectBoot(rescue, &port)
	if _, ok := actResult.(actionComplete); ok {
		actResult := hsm.reconciler.actionImageInstalling()
		if _, ok := actResult.(actionComplete); ok {
			hsm.nextState = infrav1.StateProvisioning
		}
	}
	return actResult
}

func (hsm *hostStateMachine) handleProvisioning() actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	port := hsm.host.Spec.Status.SSHSpec.PortAfterInstallImage
	actResult := hsm.reconciler.actionEnsureCorrectBoot(hsm.host.Spec.ConsumerRef.Name, &port)
	if _, ok := actResult.(actionComplete); ok {
		actResult := hsm.reconciler.actionProvisioning()
		if _, ok := actResult.(actionComplete); ok {
			hsm.nextState = infrav1.StateEnsureProvisioned
		}
	}
	return actResult
}

func (hsm *hostStateMachine) handleEnsureProvisioned() actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}

	port := hsm.host.Spec.Status.SSHSpec.PortAfterCloudInit
	actResult := hsm.reconciler.actionEnsureCorrectBoot(hsm.host.Spec.ConsumerRef.Name, &port)
	if _, ok := actResult.(actionComplete); ok {
		actResult := hsm.reconciler.actionEnsureProvisioned()
		if _, ok := actResult.(actionComplete); ok {
			hsm.nextState = infrav1.StateProvisioned
		}
	}
	return actResult
}

func (hsm *hostStateMachine) handleProvisioned() actionResult {
	if hsm.provisioningCancelled() {
		hsm.nextState = infrav1.StateDeprovisioning
		return actionComplete{}
	}
	return hsm.reconciler.actionProvisioned()
}

func (hsm *hostStateMachine) handleDeprovisioning() actionResult {
	hsm.reconciler.actionDeprovisioning()
	hsm.nextState = infrav1.StateAvailable
	return actionComplete{}
}

func (hsm *hostStateMachine) provisioningCancelled() bool {
	return hsm.host.Spec.Status.InstallImage == nil
}
