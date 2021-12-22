package inventory

import (
	"fmt"

	"github.com/go-logr/logr"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/provisioner"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// hostStateMachine is a finite state machine that manages transitions between
// the states of a BareMetalHost.
type hostStateMachine struct {
	Host        *infrav1.HetznerBareMetalHost
	NextState   infrav1.ProvisioningState
	Provisioner provisioner.Provisioner
	haveCreds   bool
}

func NewHostStateMachine(host *infrav1.HetznerBareMetalHost,
	provisioner provisioner.Provisioner,
	haveCreds bool) *hostStateMachine {
	currentState := host.Status.Provisioning.State
	r := hostStateMachine{
		Host:        host,
		NextState:   currentState, // Remain in current state by default
		Provisioner: provisioner,
		haveCreds:   haveCreds,
	}
	return &r
}

// Instead of passing a zillion arguments to the action of a phase,
// hold them in a context
type ReconcileInfo struct {
	log               logr.Logger
	host              *infrav1.HetznerBareMetalHost
	request           ctrl.Request
	bmcCredsSecret    *corev1.Secret
	events            []corev1.Event
	errorMessage      string
	postSaveCallbacks []func()
}

func NewReconcileInfo(host *infrav1.HetznerBareMetalHost) *ReconcileInfo {
	return &ReconcileInfo{
		host: host,
	}
}

type stateHandler func(*ReconcileInfo) actionResult

func (hsm *hostStateMachine) handlers() map[infrav1.ProvisioningState]stateHandler {
	return map[infrav1.ProvisioningState]stateHandler{
		infrav1.StateNone: hsm.handleNone,
	}
}

func (hsm *hostStateMachine) ReconcileState(info *ReconcileInfo) (actionRes actionResult) {
	initialState := hsm.Host.Status.Provisioning.State

	if stateHandler, found := hsm.handlers()[initialState]; found {
		return stateHandler(info)
	}

	info.log.Info("No handler found for state", "state", initialState)
	return actionError{fmt.Errorf("No handler found for state \"%s\"", initialState)}
}

func (hsm *hostStateMachine) handleNone(info *ReconcileInfo) actionResult {
	// No state is set, so immediately move to either Registering or Unmanaged
	return actionComplete{}
}
