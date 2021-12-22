package host

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/hrobot-go/models"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	hoursBeforeDeletion      time.Duration = 36
	rateLimitTimeOut         time.Duration = 660
	rateLimitTimeOutDeletion time.Duration = 120
	sshTimeOut               time.Duration = 5 * time.Second
)

// Service defines struct with machine scope to reconcile Hcloud machines.
type Service struct {
	scope *scope.BareMetalHostScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalHostScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Instead of passing a zillion arguments to the action of a phase,
// hold them in a context
type reconcileInfo struct {
	log               logr.Logger
	request           ctrl.Request
	errorMessage      string
	postSaveCallbacks []func()
}

// Reconcile implements reconcilement of Hcloud machines.
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {

	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal host", "name", s.scope.HetznerBareMetalHost.Name)

	initialState := s.scope.HetznerBareMetalHost.Spec.Status.ProvisioningState

	info := &reconcileInfo{
		log: log,
	}

	hostStateMachine := newHostStateMachine(s.scope.HetznerBareMetalHost, s)
	actResult := hostStateMachine.ReconcileState(info)
	_, err = actResult.Result() // result, err :=
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("action %q failed", initialState))
		return nil, err
	}

	return nil, nil
}

// Delete implements delete method of bare metal hosts.
func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {

	return nil, nil
}

// SetErrorMessage updates the ErrorMessage in the host Status struct
// and increases the ErrorCount
func SetErrorMessage(host *infrav1.HetznerBareMetalHost, errType infrav1.ErrorType, message string) {
	host.Spec.Status.OperationalStatus = infrav1.OperationalStatusError
	host.Spec.Status.ErrorType = errType
	host.Spec.Status.ErrorMessage = message
	host.Spec.Status.ErrorCount++
}

func recordActionFailure(scope *scope.BareMetalHostScope, errorType infrav1.ErrorType, errorMessage string) actionFailed {

	SetErrorMessage(scope.HetznerBareMetalHost, errorType, errorMessage)

	return actionFailed{dirty: true, ErrorType: errorType, errorCount: scope.HetznerBareMetalHost.Spec.Status.ErrorCount}
}

// clearError removes any existing error message.
func clearError(host *infrav1.HetznerBareMetalHost) (dirty bool) {
	host.Spec.Status.OperationalStatus = infrav1.OperationalStatusOK
	var emptyErrType infrav1.ErrorType
	if host.Spec.Status.ErrorType != emptyErrType {
		host.Spec.Status.ErrorType = emptyErrType
		dirty = true
	}
	if host.Spec.Status.ErrorMessage != "" {
		host.Spec.Status.ErrorMessage = ""
		dirty = true
	}
	return dirty
}

func (s *Service) actionNone(info *reconcileInfo) actionResult {
	if _, err := s.scope.Provisioner.GetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID); err != nil {
		if models.IsError(err, models.ErrorCodeServerNotFound) {
			return recordActionFailure(
				s.scope,
				infrav1.RegistrationError,
				fmt.Sprintf("bare metal host with id %v not found", s.scope.HetznerBareMetalHost.Spec.ServerID),
			)
		}
		return actionError{err: errors.Wrap(err, "failed to get bare metal server")}
	}
	_, err := s.scope.Provisioner.ListSSHKeys() // hetznerSSHKeys, err :=
	if err != nil {
		if models.IsError(err, models.ErrorCodeNotFound) {
			return recordActionFailure(s.scope, infrav1.RegistrationError, "no ssh key found")
		}
		return actionError{err: errors.Wrap(err, "failed to list ssh heys")}
	}
	// TODO: check whether SSH keys for machine are valid

	s.scope.SetOperationalStatus(infrav1.OperationalStatusDiscovered)
	s.scope.SetErrorCount(0)
	clearError(s.scope.HetznerBareMetalHost)
	return actionComplete{}
}
