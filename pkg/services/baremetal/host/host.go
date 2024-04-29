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

// Package host manages the state and reconcilement of bare metal host objects.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stoewer/go-strcase"
	"github.com/syself/hrobot-go/models"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

const (
	rebootWaitTime       time.Duration = 15 * time.Second
	sshResetTimeout      time.Duration = 5 * time.Minute
	softwareResetTimeout time.Duration = 5 * time.Minute
	hardwareResetTimeout time.Duration = 60 * time.Minute
	rescue               string        = "rescue"
	rescuePort           int           = 22
	gbToMebiBytes        int           = 1000
	gbToBytes            int           = 1000000 * gbToMebiBytes
	kikiToMebiBytes      int           = 1024

	errMsgFailedReboot                 = "failed to reboot bare metal server: %w"
	errMsgInvalidSSHStdOut             = "invalid output in stdOut: %w"
	errMsgFailedHandlingIncompleteBoot = "failed to handle incomplete boot: %w"
	rebootServerStr                    = "RebootBMServer"
)

var (
	errActionFailure        = fmt.Errorf("action failure")
	errNilSSHSecret         = fmt.Errorf("ssh secret is nil")
	errWrongSSHKey          = fmt.Errorf("wrong ssh key")
	errSSHConnectionRefused = fmt.Errorf("ssh connection refused")
	errUnexpectedErrorType  = fmt.Errorf("unexpected error type")
	errSSHGetHostname       = fmt.Errorf("failed to get hostname via ssh")
	errEmptyHostName        = fmt.Errorf("hostname is empty")
	errUnexpectedHostName   = fmt.Errorf("unexpected hostname")
	errMissingStorageDevice = fmt.Errorf("missing storage device")
	errUnknownRota          = fmt.Errorf("unknown rota")
	errSSHStderr            = fmt.Errorf("ssh cmd returned non-empty StdErr")
)

// Service defines struct with machine scope to reconcile HetznerBareMetalHosts.
type Service struct {
	scope *scope.BareMetalHostScope
}

// NewService outs a new service with machine scope.
func NewService(s *scope.BareMetalHostScope) *Service {
	return &Service{
		scope: s,
	}
}

// Reconcile implements reconcilement of HetznerBareMetalHosts.
func (s *Service) Reconcile(ctx context.Context) (result reconcile.Result, err error) {
	initialState := s.scope.HetznerBareMetalHost.Spec.Status.ProvisioningState

	oldHost := s.scope.HetznerBareMetalHost.DeepCopy()

	if !s.scope.HetznerBareMetalHost.DeletionTimestamp.IsZero() {
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.HostReadyCondition,
			infrav1.DeletionInProgressReason,
			clusterv1.ConditionSeverityWarning,
			"Host is not ready because it is being deleted",
		)
	}

	hostStateMachine := newHostStateMachine(s.scope.HetznerBareMetalHost, s, s.scope.Logger)

	defer func() {
		// remove deprecated conditions
		conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.DeprecatedHetznerBareMetalHostReadyCondition)
		conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.DeprecatedHostProvisionSucceededCondition)
		conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.DeprecatedRateLimitExceededCondition)
		conditions.SetSummary(s.scope.HetznerBareMetalHost)

		// save host if it changed during reconciliation
		if !reflect.DeepEqual(oldHost, s.scope.HetznerBareMetalHost) {
			saveResult, saveErr := SaveHostAndReturn(ctx, s.scope.Client, s.scope.HetznerBareMetalHost)
			err = errors.Join(err, saveErr)
			if err != nil {
				result = reconcile.Result{}
			} else if saveResult.Requeue {
				result = saveResult
			}
		}
	}()

	// reconcile state
	actResult := hostStateMachine.ReconcileState()

	result, err = actResult.Result()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("action %q failed: %w", initialState, err)
	}

	return result, nil
}

func (s *Service) recordActionFailure(errorType infrav1.ErrorType, errorMessage string) actionFailed {
	s.scope.HetznerBareMetalHost.SetError(errorType, errorMessage)
	s.scope.Error(errActionFailure, errorMessage, "errorType", errorType)
	return actionFailed{ErrorType: errorType, errorCount: s.scope.HetznerBareMetalHost.Spec.Status.ErrorCount}
}

// SaveHostAndReturn saves host object, updates LastUpdated in host status and returns the reconcile Result.
func SaveHostAndReturn(ctx context.Context, cl client.Client, host *infrav1.HetznerBareMetalHost) (res reconcile.Result, err error) {
	t := metav1.Now()
	host.Spec.Status.LastUpdated = &t

	if err := cl.Update(ctx, host); err != nil {
		if apierrors.IsConflict(err) {
			log := ctrl.LoggerFrom(ctx)
			log.V(1).Info("conflict error. Retrying", "err", err)
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to update host object: %w", err)
	}
	return res, nil
}

func (s *Service) actionPreparing() actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StatePreparing)

	server, err := s.scope.RobotClient.GetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		s.handleRobotRateLimitExceeded(err, "GetBMServer")
		if models.IsError(err, models.ErrorCodeServerNotFound) {
			msg := "bare metal host not found"
			conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.ServerNotFoundReason,
				clusterv1.ConditionSeverityError,
				msg,
			)
			s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
			return actionStop{}
		}
		return actionError{err: fmt.Errorf("failed to get bare metal server: %w", err)}
	}

	s.scope.HetznerBareMetalHost.Spec.Status.IPv4 = server.ServerIP
	s.scope.HetznerBareMetalHost.Spec.Status.IPv6 = server.ServerIPv6Net + "1"

	sshKey, actResult := s.ensureSSHKey(s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef, s.scope.RescueSSHSecret)
	if _, isComplete := actResult.(actionComplete); !isComplete {
		return actResult
	}

	s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey = &sshKey

	// Populate reboot methods in status
	if len(s.scope.HetznerBareMetalHost.Spec.Status.RebootTypes) == 0 {
		reboot, err := s.scope.RobotClient.GetReboot(s.scope.HetznerBareMetalHost.Spec.ServerID)
		if err != nil {
			s.handleRobotRateLimitExceeded(err, "GetReboot")
			return actionError{err: fmt.Errorf("failed to get reboot: %w", err)}
		}

		rebootTypes, err := rebootTypesFromStringList(reboot.Type)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to unmarshal: %w", err)}
		}
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTypes = rebootTypes
	}

	// if there is no rescue system, we cannot provision the server
	if !server.Rescue {
		errMsg := fmt.Sprintf("bm server %v has no rescue system", server.ServerNumber)
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.RescueSystemUnavailableReason,
			clusterv1.ConditionSeverityError,
			errMsg,
		)
		record.Warnf(s.scope.HetznerBareMetalHost, "NoRescueSystemAvailable", errMsg)
		s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, errMsg)
		return actionStop{}
	}

	if err := s.enforceRescueMode(); err != nil {
		return actionError{err: fmt.Errorf("failed to enforce rescue mode: %w", err)}
	}

	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})

	// Check hostname with sshClient
	out := sshClient.GetHostName()
	if trimLineBreak(out.StdOut) != "" {
		// we managed access with ssh - we can do an ssh reboot
		if err := handleSSHError(sshClient.Reboot()); err != nil {
			return actionError{err: fmt.Errorf("failed to reboot server via ssh: %w", err)}
		}
		msg := "Rebooting into rescue mode."
		createSSHRebootEvent(s.scope.HetznerBareMetalHost, msg)
		// we immediately set an error message in the host status to track the reboot we just performed
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTriggered, fmt.Sprintf("Phase %s, reboot via ssh: %s",
			s.scope.HetznerBareMetalHost.Spec.Status.ProvisioningState, msg))
		return actionComplete{}
	}

	// Check if software reboot is available. If it is not, choose hardware reboot.
	rebootType, errorType := rebootAndErrorTypeAfterTimeout(s.scope.HetznerBareMetalHost)

	if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
		s.handleRobotRateLimitExceeded(err, rebootServerStr)
		return actionError{err: fmt.Errorf(errMsgFailedReboot, err)}
	}

	msg := createRebootEvent(s.scope.HetznerBareMetalHost, rebootType, "Reboot into rescue system.")
	// we immediately set an error message in the host status to track the reboot we just performed
	s.scope.HetznerBareMetalHost.SetError(errorType, msg)
	return actionComplete{}
}

func (s *Service) enforceRescueMode() error {
	// delete old rescue activations if exist, as the ssh key might have changed in between
	if _, err := s.scope.RobotClient.DeleteBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID); err != nil {
		s.handleRobotRateLimitExceeded(err, "DeleteBootRescue")
		return fmt.Errorf("failed to delete boot rescue: %w", err)
	}
	// Rescue system is still not active - activate again
	if _, err := s.scope.RobotClient.SetBootRescue(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey.Fingerprint,
	); err != nil {
		s.handleRobotRateLimitExceeded(err, "SetBootRescue")
		return fmt.Errorf("failed to set boot rescue: %w", err)
	}
	return nil
}

func rebootTypesFromStringList(rebootTypeStringList []string) ([]infrav1.RebootType, error) {
	var rebootTypes []infrav1.RebootType
	b, err := json.Marshal(rebootTypeStringList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	if err := json.Unmarshal(b, &rebootTypes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return rebootTypes, nil
}

func (s *Service) ensureSSHKey(sshSecretRef infrav1.SSHSecretRef, sshSecret *corev1.Secret) (infrav1.SSHKey, actionResult) {
	if sshSecret == nil {
		return infrav1.SSHKey{}, actionError{err: errNilSSHSecret}
	}
	hetznerSSHKeys, err := s.scope.RobotClient.ListSSHKeys()
	if err != nil {
		s.handleRobotRateLimitExceeded(err, "ListSSHKeys")
		if !models.IsError(err, models.ErrorCodeNotFound) {
			return infrav1.SSHKey{}, actionError{err: fmt.Errorf("failed to list ssh heys: %w", err)}
		}
	}

	foundSSHKey := false
	var sshKey infrav1.SSHKey
	for _, hetznerSSHKey := range hetznerSSHKeys {
		if strings.TrimSuffix(string(sshSecret.Data[sshSecretRef.Key.Name]), "\n") == hetznerSSHKey.Name {
			foundSSHKey = true
			sshKey.Name = hetznerSSHKey.Name
			sshKey.Fingerprint = hetznerSSHKey.Fingerprint
		}
	}

	// Upload SSH key if not found
	if !foundSSHKey {
		publicKey := string(sshSecret.Data[sshSecretRef.Key.PublicKey])
		hetznerSSHKey, err := s.scope.RobotClient.SetSSHKey(string(sshSecret.Data[sshSecretRef.Key.Name]), publicKey)
		if err != nil {
			s.handleRobotRateLimitExceeded(err, "SetSSHKey")
			if models.IsError(err, models.ErrorCodeKeyAlreadyExists) {
				msg := fmt.Sprintf("cannot upload ssh key %q (from secret %q) - exists already under a different name: %s",
					string(sshSecret.Data[sshSecretRef.Key.Name]), sshSecretRef.Name, err.Error())
				conditions.MarkFalse(
					s.scope.HetznerBareMetalHost,
					infrav1.CredentialsAvailableCondition,
					infrav1.SSHKeyAlreadyExistsReason,
					clusterv1.ConditionSeverityError,
					msg,
				)
				record.Warnf(s.scope.HetznerBareMetalHost, infrav1.SSHKeyAlreadyExistsReason, msg)
				return infrav1.SSHKey{}, s.recordActionFailure(infrav1.PreparationError, msg)
			}
			return infrav1.SSHKey{}, actionError{err: fmt.Errorf("failed to set ssh key: %w", err)}
		}

		sshKey.Name = hetznerSSHKey.Name
		sshKey.Fingerprint = hetznerSSHKey.Fingerprint
	}
	return sshKey, actionComplete{}
}

func (s *Service) handleIncompleteBoot(isRebootIntoRescue, isTimeout, isConnectionRefused bool) (failed bool, err error) {
	// Connection refused error might be a sign that the ssh port is wrong - but might also come
	// right after a reboot and is expected then. Therefore, we wait for some time and if the
	// error keeps coming, we give an error.
	if isConnectionRefused {
		if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
			// if error has occurred before, check the timeout
			if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, time.Minute) {
				msg := "Connection error when targeting server with ssh that might be due to a wrong ssh port. Please check."
				if isRebootIntoRescue {
					msg = "Connection error. Can't reach rescue system via ssh."
				}
				conditions.MarkFalse(
					s.scope.HetznerBareMetalHost,
					infrav1.ProvisionSucceededCondition,
					infrav1.SSHConnectionRefusedReason,
					clusterv1.ConditionSeverityError,
					msg,
				)
				record.Warnf(s.scope.HetznerBareMetalHost, "SSHConnectionError", msg)
				return true, fmt.Errorf("%w - might be due to wrong port", errSSHConnectionRefused)
			}
		} else {
			// set error in host status to check for a timeout next time
			s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeConnectionError, "ssh gave connection error")
		}
		return false, nil
	}

	// ssh gave no connection refused error but it is still saved in host status - we can remove it
	if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
		s.scope.HetznerBareMetalHost.ClearError()
	}

	// Check whether there has been an error message already, meaning that the reboot did not finish in time.
	// Then take action accordingly. For example, if a reboot via ssh timed out, we opt for a (software) reboot
	// via API instead. If a software reboot fails / takes too long, then we trigger a hardware reboot.
	var emptyErrorType infrav1.ErrorType
	switch s.scope.HetznerBareMetalHost.Spec.Status.ErrorType {
	case emptyErrorType:
		if isTimeout {
			// A timeout error from SSH indicates that the server did not yet finish rebooting.
			// As the sevrer has no error set yet, set error message and return.
			s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTriggered, "ssh timeout error - server has not restarted yet")
			return false, nil
		}

		// We did not get an error with ssh - but also not the expected hostname. Therefore,
		// the (ssh) reboot did not start. We trigger an API reboot instead.
		return false, s.handleErrorTypeSSHRebootFailed(isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeSSHRebootTriggered:
		return false, s.handleErrorTypeSSHRebootFailed(isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeSoftwareRebootTriggered:
		return false, s.handleErrorTypeSoftwareRebootFailed(isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeHardwareRebootTriggered:
		return false, s.handleErrorTypeHardwareRebootFailed(isTimeout, isRebootIntoRescue)
	}

	return false, fmt.Errorf("%w: %s", errUnexpectedErrorType, s.scope.HetznerBareMetalHost.Spec.Status.ErrorType)
}

func (s *Service) handleErrorTypeSSHRebootFailed(isSSHTimeoutError, wantsRescue bool) error {
	// If it is not a timeout error, then the ssh command (get hostname) worked, but didn't give us the
	// right hostname. This means that the server has not been rebooted and we need to escalate.
	// If we got a timeout error from ssh, it means that the server has not yet finished rebooting.
	// If the timeout for ssh reboots has been reached, then escalate.
	rebootInto := "node"
	if wantsRescue {
		rebootInto = "rescue mode"
	}
	if !isSSHTimeoutError || hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, sshResetTimeout) {
		if wantsRescue {
			// make sure hat we boot into rescue mode if that is necessary
			if err := s.ensureRescueMode(); err != nil {
				return fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}

		// Check if software reboot is available. If it is not, choose hardware reboot.
		rebootType, errorType := rebootAndErrorTypeAfterTimeout(s.scope.HetznerBareMetalHost)

		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
			s.handleRobotRateLimitExceeded(err, rebootServerStr)
			return fmt.Errorf(errMsgFailedReboot, err)
		}
		msg := fmt.Sprintf("Reboot via ssh into %s failed. Now using rebootType %q.",
			rebootInto, rebootType)
		msg = createRebootEvent(s.scope.HetznerBareMetalHost, rebootType, msg)
		// we immediately set an error message in the host status to track the reboot we just performed
		s.scope.HetznerBareMetalHost.SetError(errorType, msg)
	}
	return nil
}

func rebootAndErrorTypeAfterTimeout(host *infrav1.HetznerBareMetalHost) (infrav1.RebootType, infrav1.ErrorType) {
	var rebootType infrav1.RebootType
	var errorType infrav1.ErrorType
	switch {
	case host.HasSoftwareReboot():
		rebootType = infrav1.RebootTypeSoftware
		errorType = infrav1.ErrorTypeSoftwareRebootTriggered
	case host.HasHardwareReboot():
		rebootType = infrav1.RebootTypeHardware
		errorType = infrav1.ErrorTypeHardwareRebootTriggered
	default:
		// this is very unexpected and indicates something to be seriously wrong
		panic("no software or hardware reboot available for host")
	}
	return rebootType, errorType
}

func (s *Service) handleErrorTypeSoftwareRebootFailed(isSSHTimeoutError, wantsRescue bool) error {
	rebootInto := "node"
	if wantsRescue {
		rebootInto = "rescue mode"
	}
	// If it is not a timeout error, then the ssh command (get hostname) worked, but didn't give us the
	// right hostname. This means that the server has not been rebooted and we need to escalate.
	// If we got a timeout error from ssh, it means that the server has not yet finished rebooting.
	// If the timeout for software reboots has been reached, then escalate.
	if !isSSHTimeoutError || hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, softwareResetTimeout) {
		if wantsRescue {
			// make sure hat we boot into rescue mode if that is necessary
			if err := s.ensureRescueMode(); err != nil {
				return fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}
		// Perform hardware reboot
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			s.handleRobotRateLimitExceeded(err, rebootServerStr)
			return fmt.Errorf(errMsgFailedReboot, err)
		}
		msg := fmt.Sprintf("Reboot via type 'software' into %s failed. Now using rebootType %q.",
			rebootInto, infrav1.RebootTypeHardware)
		msg = createRebootEvent(s.scope.HetznerBareMetalHost, infrav1.RebootTypeHardware, msg)
		// we immediately set an error message in the host status to track the reboot we just performed
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootTriggered, msg)
	}

	return nil
}

func (s *Service) handleErrorTypeHardwareRebootFailed(isSSHTimeoutError, wantsRescue bool) error {
	rebootInto := "node"
	if wantsRescue {
		rebootInto = "rescue mode"
	}
	// If it is not a timeout error, then the ssh command (get hostname) worked, but didn't give us the
	// right hostname. This means that the server has not been rebooted and we need to escalate.
	// If we got a timeout error from ssh, it means that the server has not yet finished rebooting.
	// If the timeout for hardware reboots has been reached, then escalate.
	if !isSSHTimeoutError || hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		if wantsRescue {
			// make sure hat we boot into rescue mode if that is necessary
			if err := s.ensureRescueMode(); err != nil {
				return fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}

		// as we don't change the status of the host, we manually update LastUpdated
		t := metav1.Now()
		s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated = &t

		// we immediately set an error message in the host status to track the reboot we just performed
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			s.handleRobotRateLimitExceeded(err, rebootServerStr)
			return fmt.Errorf(errMsgFailedReboot, err)
		}
		msg := fmt.Sprintf("Reboot via ssh into %s failed. Now using rebootType %q.",
			rebootInto, infrav1.RebootTypeHardware)
		createRebootEvent(s.scope.HetznerBareMetalHost, infrav1.RebootTypeHardware, msg)
	}
	return nil
}

func hasTimedOut(lastUpdated *metav1.Time, timeout time.Duration) bool {
	now := metav1.Now()
	return lastUpdated.Add(timeout).Before(now.Time)
}

func (s *Service) ensureRescueMode() error {
	rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		s.handleRobotRateLimitExceeded(err, "GetBootRescue")
		return fmt.Errorf("failed to get boot rescue: %w", err)
	}
	if !rescue.Active {
		// Rescue system is still not active - activate again
		if _, err := s.scope.RobotClient.SetBootRescue(
			s.scope.HetznerBareMetalHost.Spec.ServerID,
			s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey.Fingerprint,
		); err != nil {
			s.handleRobotRateLimitExceeded(err, "SetBootRescue")
			return fmt.Errorf("failed to set boot rescue: %w", err)
		}
	}
	return nil
}

func (s *Service) actionRegistering() actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateRegistering)

	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	// Check hostname with sshClient
	out := sshClient.GetHostName()
	hostName := trimLineBreak(out.StdOut)
	if hostName != rescue {
		// give the reboot some time until it takes effect
		if s.hasJustRebooted() {
			return actionContinue{delay: 2 * time.Second}
		}

		isSSHTimeoutError, isSSHConnectionRefusedError, err := s.analyzeSSHOutputRegistering(out)
		if err != nil {
			// This can happen if the bare-metal server was taken by another mgt-cluster.
			// Check in https://robot.hetzner.com/server for the "History" of the server.
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - registering: %w", err)}
		}

		failed, err := s.handleIncompleteBoot(true, isSSHTimeoutError, isSSHConnectionRefusedError)
		if failed {
			return s.recordActionFailure(infrav1.ProvisioningError, err.Error())
		}
		if err != nil {
			return actionError{err: fmt.Errorf(errMsgFailedHandlingIncompleteBoot, err)}
		}
		timeSinceReboot := "unknown"
		if s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated != nil {
			timeSinceReboot = time.Since(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated.Time).String()
		}
		s.scope.Logger.Info("Could not reach rescue system. Will retry some seconds later.", "stdout", out.StdOut, "stderr", out.StdErr, "err", out.Err.Error(),
			"isSSHTimeoutError", isSSHTimeoutError, "isSSHConnectionRefusedError", isSSHConnectionRefusedError, "timeSinceReboot", timeSinceReboot)
		return actionContinue{delay: 10 * time.Second}
	}

	output := sshClient.GetHardwareDetailsDebug()
	if output.Err != nil {
		return actionError{err: fmt.Errorf("failed to obtain hardware for debugging: %w", output.Err)}
	}
	msg := fmt.Sprintf("%s\n\n", output.StdOut)
	if out.StdErr != "" {
		msg += fmt.Sprintf("stderr:\n%s\n\n", out.StdErr)
	}
	record.Eventf(s.scope.HetznerBareMetalHost, "GetHardwareDetails", msg)

	if s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails == nil {
		hardwareDetails, err := getHardwareDetails(sshClient)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to get hardware details: %w", err)}
		}
		s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails = &hardwareDetails
	}

	if s.scope.HetznerBareMetalHost.Spec.RootDeviceHints == nil {
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1.ConditionSeverityError,
			infrav1.ErrorMessageMissingRootDeviceHints,
		)
		return s.recordActionFailure(infrav1.RegistrationError, infrav1.ErrorMessageMissingRootDeviceHints)
	}
	errMsg := s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.IsValidWithMessage()
	if errMsg != "" {
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1.ConditionSeverityError,
			errMsg,
		)
		return s.recordActionFailure(infrav1.RegistrationError, errMsg)
	}

	if err := validateRootDeviceWwnsAreSubsetOfExistingWwns(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints,
		s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails.Storage); err != nil {
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1.ConditionSeverityError,
			err.Error(),
		)
		return s.recordActionFailure(infrav1.RegistrationError, err.Error())
	}

	// Check RAID for the second time.
	// See "tworaidchecks" for the other place.
	msg = ""
	if s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Swraid != 0 &&
		len(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.Raid.WWN) < 2 {
		msg = "Invalid HetznerBareMetalHost: spec.status.installImage.swraid is active. Use at least two WWNs in spec.rootDevideHints.raid.wwn."
	} else if s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Swraid == 0 &&
		s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.WWN == "" {
		msg = "Invalid HetznerBareMetalHost: spec.status.installImage.swraid is not active. Use spec.rootDevideHints.wwn and leave raid.wwn empty."
	}
	if msg != "" {
		// This triggers a FailureMessage on the HetznerBareMetalMachine
		// and CAPI machine and will lead to this Machine to be deleted.
		// Another machine (with same swraid setting) will not take the same host anymore,
		// because the rootDeviceHints don't fit.
		s.scope.Logger.Info(msg)
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1.ConditionSeverityError,
			msg,
		)
		return s.recordActionFailure(infrav1.FatalError, msg)
	}

	conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RootDeviceHintsValidatedCondition)
	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints *infrav1.RootDeviceHints, storageDevices []infrav1.Storage) error {
	knownWWNs := make([]string, 0, len(storageDevices))
	for _, sd := range storageDevices {
		knownWWNs = append(knownWWNs, sd.WWN)
	}

	for _, wwn := range rootDeviceHints.ListOfWWN() {
		if slices.Contains(knownWWNs, wwn) {
			continue
		}
		return fmt.Errorf("%w for root device hint %q. Known WWNs: %v", errMissingStorageDevice, wwn, knownWWNs)
	}
	return nil
}

func getHardwareDetails(sshClient sshclient.Client) (infrav1.HardwareDetails, error) {
	mebiBytes, err := obtainHardwareDetailsRAM(sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details RAM: %w", err)
	}

	nics, err := obtainHardwareDetailsNics(sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details Nics: %w", err)
	}

	storage, err := obtainHardwareDetailsStorage(sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details storage: %w", err)
	}

	// remove names of storage devices because they might change
	for _, device := range storage {
		device.Name = ""
	}

	cpu, err := obtainHardwareDetailsCPU(sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details CPU: %w", err)
	}

	return infrav1.HardwareDetails{
		RAMGB:   mebiBytes / gbToMebiBytes,
		NIC:     nics,
		Storage: storage,
		CPU:     cpu,
	}, nil
}

func (s *Service) analyzeSSHOutputRegistering(out sshclient.Output) (isSSHTimeoutError, isConnectionRefused bool, reterr error) {
	if out.Err != nil {
		return s.analyzeSSHErrorRegistering(out.Err)
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		return false, false, fmt.Errorf("%w: StdErr: %s", errSSHGetHostname, out.StdErr)
	}

	if trimLineBreak(out.StdOut) == "" {
		// Hostname should not be empty. This is unexpected.
		return false, false, errEmptyHostName
	}

	// wrong hostname
	return false, false, nil
}

func (s *Service) analyzeSSHErrorRegistering(sshErr error) (isSSHTimeoutError, isConnectionRefused bool, reterr error) {
	// check if the reboot triggered
	rebootTriggered, err := s.rebootTriggered()
	if err != nil {
		return false, false, fmt.Errorf("failed to check whether reboot triggered: %w", err)
	}

	switch {
	case os.IsTimeout(sshErr) || sshclient.IsTimeoutError(sshErr):
		isSSHTimeoutError = true
	case sshclient.IsAuthenticationFailedError(sshErr):
		if !rebootTriggered {
			return false, false, nil
		}
		reterr = fmt.Errorf("wrong ssh key: %w", sshErr)
	case sshclient.IsConnectionRefusedError(sshErr):
		if !rebootTriggered && s.scope.HetznerBareMetalHost.Spec.Status.ErrorType != infrav1.ErrorTypeHardwareRebootTriggered {
			// Reboot did not trigger
			return false, false, nil
		}
		isConnectionRefused = true

	default:
		reterr = fmt.Errorf("unhandled ssh error while getting hostname: %w", sshErr)
	}
	return isSSHTimeoutError, isConnectionRefused, reterr
}

func (s *Service) rebootTriggered() (bool, error) {
	rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		s.handleRobotRateLimitExceeded(err, "GetBootRescue")
		return false, fmt.Errorf("failed to get boot rescue: %w", err)
	}
	return !rescue.Active, nil
}

func obtainHardwareDetailsRAM(sshClient sshclient.Client) (int, error) {
	out := sshClient.GetHardwareDetailsRAM()
	if err := handleSSHError(out); err != nil {
		return 0, err
	}
	stdOut := trimLineBreak(out.StdOut)
	if stdOut == "" {
		return 0, sshclient.ErrEmptyStdOut
	}

	kibiBytes, err := strconv.Atoi(stdOut)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ssh output to memory int. StdOut %s: %w", stdOut, err)
	}
	mebiBytes := kibiBytes / kikiToMebiBytes

	return mebiBytes, nil
}

func obtainHardwareDetailsNics(sshClient sshclient.Client) ([]infrav1.NIC, error) {
	type originalNic struct {
		Name      string `json:"name,omitempty"`
		Model     string `json:"model,omitempty"`
		MAC       string `json:"mac,omitempty"`
		IP        string `json:"ip,omitempty"`
		SpeedMbps string `json:"speedMbps,omitempty"`
	}

	out := sshClient.GetHardwareDetailsNics()
	if err := handleSSHError(out); err != nil {
		return nil, err
	}
	stdOut := trimLineBreak(out.StdOut)
	if stdOut == "" {
		return nil, sshclient.ErrEmptyStdOut
	}

	stringArray := strings.Split(stdOut, "\n")
	nicsArray := make([]infrav1.NIC, len(stringArray))

	for i, str := range stringArray {
		validJSONString := validJSONFromSSHOutput(str)

		var nic originalNic
		if err := json.Unmarshal([]byte(validJSONString), &nic); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %v. Original ssh output %s: %w", validJSONString, stdOut, err)
		}
		speedMbps, err := strconv.Atoi(nic.SpeedMbps)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int from string %s: %w", nic.SpeedMbps, err)
		}
		nicsArray[i] = infrav1.NIC{
			Name:      nic.Name,
			Model:     nic.Model,
			MAC:       nic.MAC,
			IP:        nic.IP,
			SpeedMbps: speedMbps,
		}
	}

	return nicsArray, nil
}

func obtainHardwareDetailsStorage(sshClient sshclient.Client) ([]infrav1.Storage, error) {
	type originalStorage struct {
		Name         string `json:"name,omitempty"`
		Type         string `json:"type,omitempty"`
		FsType       string `json:"fsType,omitempty"`
		Label        string `json:"label,omitempty"`
		SizeBytes    string `json:"size,omitempty"`
		Vendor       string `json:"vendor,omitempty"`
		Model        string `json:"model,omitempty"`
		SerialNumber string `json:"serial,omitempty"`
		WWN          string `json:"wwn,omitempty"`
		HCTL         string `json:"hctl,omitempty"`
		Rota         string `json:"rota,omitempty"`
	}

	out := sshClient.GetHardwareDetailsStorage()
	if err := handleSSHError(out); err != nil {
		return nil, err
	}
	stdOut := trimLineBreak(out.StdOut)
	if stdOut == "" {
		return nil, sshclient.ErrEmptyStdOut
	}

	stringArray := strings.Split(stdOut, "\n")
	storageArray := make([]infrav1.Storage, 0, len(stringArray))

	for _, str := range stringArray {
		validJSONString := validJSONFromSSHOutput(str)

		var storage originalStorage
		if err := json.Unmarshal([]byte(validJSONString), &storage); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %v. Original ssh output %s: %w", validJSONString, stdOut, err)
		}
		sizeBytes, err := strconv.Atoi(storage.SizeBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int from string %s: %w", storage.SizeBytes, err)
		}

		var rota bool
		switch storage.Rota {
		case "1":
			rota = true
		case "0":
			rota = false
		default:
			return nil, fmt.Errorf("%w: Got %s. Expect either 1 or 0", errUnknownRota, storage.Rota)
		}

		sizeGB := sizeBytes / gbToBytes
		capacityGB := infrav1.Capacity(sizeGB)

		if storage.Type == "disk" {
			storageArray = append(storageArray, infrav1.Storage{
				Name:         storage.Name,
				SizeBytes:    infrav1.Capacity(sizeBytes),
				SizeGB:       capacityGB,
				Vendor:       strings.TrimSpace(storage.Vendor),
				Model:        storage.Model,
				SerialNumber: storage.SerialNumber,
				WWN:          storage.WWN,
				HCTL:         storage.HCTL,
				Rota:         rota,
			})
		}
	}

	return storageArray, nil
}

func obtainHardwareDetailsCPU(sshClient sshclient.Client) (cpu infrav1.CPU, err error) {
	cpu.Arch, err = getCPUArch(sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU arch: %w", err)
	}

	cpu.Model, err = getCPUModel(sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU model: %w", err)
	}

	cpu.ClockGigahertz, err = getCPUClockGigahertz(sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU clock speed: %w", err)
	}

	cpu.Threads, err = getCPUThreads(sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU threads: %w", err)
	}

	cpu.Flags, err = getCPUFlags(sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU flags: %w", err)
	}

	return cpu, nil
}

func getCPUArch(sshClient sshclient.Client) (string, error) {
	out := sshClient.GetHardwareDetailsCPUArch()
	if err := handleSSHError(out); err != nil {
		return "", err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return "", fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}

	return stdOut, nil
}

func getCPUModel(sshClient sshclient.Client) (string, error) {
	out := sshClient.GetHardwareDetailsCPUModel()
	if err := handleSSHError(out); err != nil {
		return "", err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return "", fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}
	return stdOut, nil
}

func getCPUClockGigahertz(sshClient sshclient.Client) (infrav1.ClockSpeed, error) {
	out := sshClient.GetHardwareDetailsCPUClockGigahertz()
	if err := handleSSHError(out); err != nil {
		return infrav1.ClockSpeed(""), err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return infrav1.ClockSpeed(""), fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}

	return infrav1.ClockSpeed(stdOut), nil
}

func getCPUThreads(sshClient sshclient.Client) (int, error) {
	out := sshClient.GetHardwareDetailsCPUThreads()
	if err := handleSSHError(out); err != nil {
		return 0, err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return 0, fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}

	threads, err := strconv.Atoi(stdOut)
	if err != nil {
		return 0, fmt.Errorf("failed to parse string to int. Stdout %s: %w", stdOut, err)
	}

	return threads, nil
}

func getCPUFlags(sshClient sshclient.Client) ([]string, error) {
	out := sshClient.GetHardwareDetailsCPUFlags()
	if err := handleSSHError(out); err != nil {
		return nil, err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return nil, fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}

	return strings.Split(stdOut, " "), nil
}

func handleSSHError(out sshclient.Output) error {
	if out.Err != nil {
		return fmt.Errorf("failed to perform ssh command: stdout %q. stderr %q. %w", out.StdOut, out.StdErr, out.Err)
	}
	if out.StdErr != "" {
		return fmt.Errorf("%w: StdErr: %s", errSSHStderr, out.StdErr)
	}
	return nil
}

func validateStdOut(stdOut string) (string, error) {
	stdOut = trimLineBreak(stdOut)
	if stdOut == "" {
		return "", sshclient.ErrEmptyStdOut
	}
	return stdOut, nil
}

func (s *Service) actionImageInstalling() actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateImageInstalling)

	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	// Ensure os ssh secret
	sshKey, actResult := s.ensureSSHKey(s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef, s.scope.OSSSHSecret)
	if _, isComplete := actResult.(actionComplete); !isComplete {
		return actResult
	}

	s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.OSKey = &sshKey

	// If there is a Linux OS on an other disk, then the reboot after the provisioning
	// will likely fail, because the machine boots into the other operating system.
	// We want detect that early, and not start the provisioning process.
	out := sshClient.DetectLinuxOnAnotherDisk(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN())
	if out.Err != nil {
		var exitErr *ssh.ExitError
		if errors.As(out.Err, &exitErr) && exitErr.ExitStatus() > 0 {
			// The script detected Linux on an other disk. This is a permanent error.
			msg := fmt.Sprintf("DetectLinuxOnAnotherDisk failed (permanent error): %s. StdErr: %s (%s)",
				out.StdOut, out.StdErr, out.Err.Error())
			conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.LinuxOnOtherDiskFoundReason,
				clusterv1.ConditionSeverityError,
				msg,
			)
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.LinuxOnOtherDiskFoundReason, msg)
			s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
			return actionStop{}
		}

		// Some other error like connection timeout. Retry again later.
		// This often during provisioning.
		msg := fmt.Sprintf("will retry: %s. StdErr: %s (%s)",
			out.StdOut, out.StdErr, out.Err.Error())
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.SSHToRescueSystemFailedReason,
			clusterv1.ConditionSeverityInfo,
			msg,
		)
		record.Event(s.scope.HetznerBareMetalHost, infrav1.SSHToRescueSystemFailedReason, msg)
		return actionContinue{
			delay: 10 * time.Second,
		}
	}
	record.Eventf(s.scope.HetznerBareMetalHost, "NoLinuxOnAnotherDisk", "OK, no Linux on another disk:\n%s\n\n%s", out.StdOut, out.StdErr)

	// if the previous reconcile was stopped, then wait until the first
	// run of installimage was finished.
	out = sshClient.GetRunningInstallImageProcesses()
	if out.Err != nil {
		return actionError{err: fmt.Errorf("failed to get running installimage processes: %q %q %w", out.StdOut, out.StdErr, out.Err)}
	}
	if out.StdOut != "" {
		record.Warnf(s.scope.HetznerBareMetalHost, "InstallImageAlreadyRunning",
			"installimage is already running:\n%s", out.StdOut)
		return actionContinue{
			delay: 10 * time.Second,
		}
	}

	record.Event(s.scope.HetznerBareMetalHost, "InstallImagePreflightCheckSuccessful", "Rescue system reachable, disks look good.")

	autoSetupInput, actionRes := s.createAutoSetupInput(sshClient)
	if actionRes != nil {
		return actionRes
	}

	autoSetup := buildAutoSetup(s.scope.HetznerBareMetalHost.Spec.Status.InstallImage, autoSetupInput)

	out = sshClient.CreateAutoSetup(autoSetup)
	if out.Err != nil {
		return actionError{err: fmt.Errorf("failed to create autosetup: %q %q %w", out.StdOut, out.StdErr, out.Err)}
	}

	if out.StdErr != "" {
		return actionError{err: fmt.Errorf("failed to create autosetup: %q %q %w. Content: %s", out.StdOut, out.StdErr, out.Err, autoSetup)}
	}

	// create post install script
	postInstallScript := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.PostInstallScript

	if postInstallScript != "" {
		if err := handleSSHError(sshClient.CreatePostInstallScript(postInstallScript)); err != nil {
			return actionError{err: fmt.Errorf("failed to create post install script %s: %w", postInstallScript, err)}
		}
	}

	record.Event(s.scope.HetznerBareMetalHost, "InstallingMachineImageStarted",
		s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image.String())

	out = sshClient.UntarTGZ()
	if out.Err != nil {
		record.Warnf(s.scope.HetznerBareMetalHost, "UntarInstallimageTgzFailed", "err: %s, stderr: %s", out.Err.Error(), out.StdErr)
		return actionError{err: fmt.Errorf("UntarInstallimageTgzFailed: %w", out.Err)}
	}
	record.Event(s.scope.HetznerBareMetalHost, "ExecuteInstallImageStarted",
		s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image.String())

	// Execute install image
	out = sshClient.ExecuteInstallImage(postInstallScript != "")
	if out.Err != nil {
		record.Warnf(s.scope.HetznerBareMetalHost, "ExecuteInstallImageFailed", out.StdOut)
		return actionError{err: fmt.Errorf("failed to execute installimage: %w", out.Err)}
	}

	record.Eventf(s.scope.HetznerBareMetalHost, "ExecuteInstallImageSucceeded", out.StdOut)
	s.scope.Logger.Info("ExecuteInstallImageSucceeded", "stdout", out.StdOut, "stderr", out.StdErr)

	// Update name in robot API
	if _, err := s.scope.RobotClient.SetBMServerName(s.scope.HetznerBareMetalHost.Spec.ServerID, autoSetupInput.hostName); err != nil {
		record.Warn(s.scope.HetznerBareMetalHost, "SetBMServerNameFailed", err.Error())
		s.handleRobotRateLimitExceeded(err, "SetBMServerName")
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	if err := handleSSHError(sshClient.Reboot()); err != nil {
		record.Warn(s.scope.HetznerBareMetalHost, "RebootFailed", err.Error())
		return actionError{err: fmt.Errorf("failed to reboot server: %w", err)}
	}
	createSSHRebootEvent(s.scope.HetznerBareMetalHost, "machine image was installed")

	s.scope.Logger.Info("RebootAfterInstallimageSucceeded", "stdout", out.StdOut, "stderr", out.StdErr)

	// clear potential errors - all done
	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func (s *Service) createAutoSetupInput(sshClient sshclient.Client) (autoSetupInput, actionResult) {
	image := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image
	imagePath, needsDownload, errorMessage := image.GetDetails()
	if errorMessage != "" {
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.ImageSpecInvalidReason,
			clusterv1.ConditionSeverityError,
			errorMessage,
		)
		return autoSetupInput{}, s.recordActionFailure(infrav1.ProvisioningError, errorMessage)
	}
	if needsDownload {
		out := sshClient.DownloadImage(imagePath, image.URL)
		if err := handleSSHError(out); err != nil {
			err := fmt.Errorf("failed to download image: %s %s %w", out.StdOut, out.StdErr, err)
			conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.ImageDownloadFailedReason,
				clusterv1.ConditionSeverityError,
				err.Error(),
			)
			return autoSetupInput{}, actionError{err: err}
		}
	}

	// get the information about storage devices again to have the latest names which are then taken for installimage
	storage, err := obtainHardwareDetailsStorage(sshClient)
	if err != nil {
		return autoSetupInput{}, actionError{err: fmt.Errorf("failed to obtain hardware details storage: %w", err)}
	}

	// get device names from storage device
	deviceNames := getDeviceNames(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN(), storage)

	// we need at least one storage device
	if len(deviceNames) == 0 {
		msg := "no suitable storage device found"
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.NoStorageDeviceFoundReason,
			clusterv1.ConditionSeverityError,
			msg,
		)
		return autoSetupInput{}, s.recordActionFailure(infrav1.ProvisioningError, msg)
	}

	// Create autosetup file
	return autoSetupInput{
		osDevices: deviceNames,
		hostName:  s.scope.Hostname(),
		image:     imagePath,
	}, nil
}

func getDeviceNames(wwn []string, storageDevices []infrav1.Storage) []string {
	deviceNames := make([]string, 0, len(storageDevices))
	for _, device := range storageDevices {
		if utils.StringInList(wwn, device.WWN) {
			deviceNames = append(deviceNames, device.Name)
		}
	}
	return deviceNames
}

func (s *Service) actionProvisioning() actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateProvisioning)

	host := s.scope.HetznerBareMetalHost

	portAfterInstallImage := host.Spec.Status.SSHSpec.PortAfterInstallImage
	privateKey := sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, host.Spec.Status.SSHSpec.SecretRef).PrivateKey
	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: privateKey,
		Port:       portAfterInstallImage,
		IP:         host.Spec.Status.GetIPAddress(),
	})

	// check hostname with sshClient
	wantHostName := s.scope.Hostname()

	out := sshClient.GetHostName()

	if out.Err != nil {
		msg := fmt.Sprintf("ssh to port %d failed: %s (%s): %v", portAfterInstallImage, out.StdOut, out.StdErr, out.Err)
		conditions.MarkFalse(host,
			infrav1.SSHAfterInstallImageSucceededCondition,
			infrav1.SSHAfterInstallImageFailedReason,
			clusterv1.ConditionSeverityWarning, msg)

		if time.Now().After(conditions.GetLastTransitionTime(
			s.scope.HetznerBareMetalHost,
			infrav1.SSHAfterInstallImageSucceededCondition).Add(7 * time.Minute)) {
			// We waited some minutes. Still no connection. There seems to something wrong.
			record.Warn(host, infrav1.SSHAfterInstallImageFailedReason, msg)
		}
	} else {
		conditions.MarkTrue(s.scope.HetznerBareMetalHost,
			infrav1.SSHAfterInstallImageSucceededCondition)
	}

	if trimLineBreak(out.StdOut) != wantHostName {
		// Give the reboot some time until it takes effect. Otherwise the ssh connection gets done too fast,
		// and it will connect to the machine before it gets rebooted.
		if s.hasJustRebooted() {
			s.scope.Logger.Info("hasJustRebooted is true", "LastUpdated", host.Spec.Status.LastUpdated)
			return actionContinue{delay: 2 * time.Second}
		}

		privateKeyRescue := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef).PrivateKey
		rescueSSHClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
			PrivateKey: privateKeyRescue,
			Port:       rescuePort,
			IP:         host.Spec.Status.GetIPAddress(),
		})

		isSSHTimeoutError, isSSHConnectionRefusedError, err := analyzeSSHOutputInstallImage(out, rescueSSHClient, portAfterInstallImage)
		if err != nil {
			if errors.Is(err, errUnexpectedHostName) {
				// One possible reason: The machine gets used by a second wl-cluster
				record.Warn(host, "UnexpectedHostName",
					fmt.Sprintf("Provisioning: wanted %q. %s", wantHostName, err.Error()))
			}
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - installImage: %w", err)}
		}
		failed, err := s.handleIncompleteBoot(false, isSSHTimeoutError, isSSHConnectionRefusedError)
		if failed {
			return s.recordActionFailure(infrav1.ProvisioningError, err.Error())
		}
		if err != nil {
			return actionError{err: fmt.Errorf(errMsgFailedHandlingIncompleteBoot, err)}
		}
		return actionContinue{delay: 10 * time.Second}
	}

	// we are in correct boot and can start provisioning
	if failedAction := s.provision(sshClient); failedAction != nil {
		return failedAction
	}

	host.ClearError()
	return actionComplete{}
}

func (s *Service) provision(sshClient sshclient.Client) actionResult {
	{
		out := sshClient.EnsureCloudInit()
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to ensure cloud init: %w", err)}
		}

		if trimLineBreak(out.StdOut) == "" {
			msg := "cloud init not installed"
			conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.CloudInitNotInstalledReason,
				clusterv1.ConditionSeverityError,
				msg,
			)
			return s.recordActionFailure(infrav1.ProvisioningError, msg)
		}
	}

	if err := handleSSHError(sshClient.CreateNoCloudDirectory()); err != nil {
		return actionError{err: fmt.Errorf("failed to create no cloud directory: %w", err)}
	}

	if err := handleSSHError(sshClient.CreateMetaData(s.scope.Hostname())); err != nil {
		return actionError{err: fmt.Errorf("failed to create meta data: %w", err)}
	}

	userData, err := s.scope.GetRawBootstrapData(context.TODO())
	if err != nil {
		return actionError{err: fmt.Errorf("failed to get user data: %w", err)}
	}

	if err := handleSSHError(sshClient.CreateUserData(string(userData))); err != nil {
		return actionError{err: fmt.Errorf("failed to create user data: %w", err)}
	}

	if err := handleSSHError(sshClient.Reboot()); err != nil {
		return actionError{err: fmt.Errorf("failed to reboot: %w", err)}
	}
	createSSHRebootEvent(s.scope.HetznerBareMetalHost, "UserData of cloud-init was created")
	return nil
}

func analyzeSSHOutputInstallImage(out sshclient.Output, sshClient sshclient.Client, port int) (isTimeout, isConnectionRefused bool, reterr error) {
	// check err
	if out.Err != nil {
		switch {
		case os.IsTimeout(out.Err) || sshclient.IsTimeoutError(out.Err):
			isTimeout = true
			return isTimeout, false, nil
		case sshclient.IsAuthenticationFailedError(out.Err):
			if err := handleAuthenticationFailed(sshClient, port); err != nil {
				return false, false, fmt.Errorf("original ssh error: %w. err: %w", out.Err, err)
			}
			return false, false, handleAuthenticationFailed(sshClient, port)
		case sshclient.IsConnectionRefusedError(out.Err):
			return false, verifyConnectionRefused(sshClient, port), nil
		}

		return false, false, fmt.Errorf("unhandled ssh error while getting hostname: %w", out.Err)
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		return false, false, fmt.Errorf("%w: StdErr: %s", errSSHGetHostname, out.StdErr)
	}

	// check stdout
	hostname := trimLineBreak(out.StdOut)
	switch hostname {
	case "":
		// Hostname should not be empty. This is unexpected.
		return false, false, errEmptyHostName
	case rescue: // We are in wrong boot, nothing has to be done to trigger reboot
		return false, false, nil
	}

	// We are in the case that hostName != rescue && StdOut != hostName
	// This is unexpected
	return false, false, fmt.Errorf("%w: %s", errUnexpectedHostName, hostname)
}

func handleAuthenticationFailed(sshClient sshclient.Client, port int) error {
	// Check whether we are in the wrong system in the case that rescue and os system might be running on the same port.
	if port == rescuePort {
		if sshClient.GetHostName().Err == nil {
			// We are in the wrong system, so return false, false, nil
			return nil
		}
	}
	return errWrongSSHKey
}

func verifyConnectionRefused(sshClient sshclient.Client, port int) bool {
	// Check whether we are in the wrong system in the case that rescue and os system might be running on the same port.
	if port != rescuePort {
		// Check whether we are in the wrong system
		if sshClient.GetHostName().Err == nil {
			// We are in the wrong system - this error is not temporary
			return false
		}
	}
	return true
}

func (s *Service) actionEnsureProvisioned() (ar actionResult) {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateEnsureProvisioned)
	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})

	defer func() {
		// Create an Event which contains the content of /var/log/cloud-init-output.log

		if _, ok := ar.(actionContinue); ok {
			// don't create an event
			return
		}
		out := sshClient.GetCloudInitOutput()
		if out.Err != nil || out.StdErr != "" {
			record.Warnf(s.scope.HetznerBareMetalHost, "GetCloudInitOutputFailed",
				fmt.Sprintf("GetCloudInitOutput failed to get /var/log/cloud-init-output.log: stdout %q, stderr %q, err %q",
					out.StdOut, out.StdErr, out.Err.Error()))
			return
		}
		_, ok := ar.(actionComplete)
		if ok {
			record.Eventf(s.scope.HetznerBareMetalHost, "CloudInitOutput",
				"/var/log/cloud-init-output.log: "+out.StdOut)
		} else {
			_, err := ar.Result()
			errString := ""
			if err != nil {
				errString = err.Error()
			}
			record.Warnf(s.scope.HetznerBareMetalHost, "CloudInitOutput", fmt.Sprintf("cloud init output (%s):\n%s",
				errString,
				out.StdOut))
		}
	}()
	// Check hostname with sshClient
	wantHostName := s.scope.Hostname()

	out := sshClient.GetHostName()
	if trimLineBreak(out.StdOut) != wantHostName {
		// give the reboot some time until it takes effect
		if s.hasJustRebooted() {
			return actionContinue{delay: 2 * time.Second}
		}

		isTimeout, isSSHConnectionRefusedError, err := analyzeSSHOutputProvisioned(out)
		if err != nil {
			if errors.Is(err, errUnexpectedHostName) {
				// One possible reason: The machine gets used by a second wl-cluster
				record.Warnf(s.scope.HetznerBareMetalHost, "UnexpectedHostName",
					"EnsureProvision: wanted %q. %s", wantHostName, err.Error())
			}
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - provisioning: %w", err)}
		}
		// A connection failed error could mean that cloud init is still running (if cloudInit introduces a new port)
		if isSSHConnectionRefusedError {
			if actionRes := s.handleConnectionRefused(); actionRes != nil {
				return actionRes
			}
		}

		failed, err := s.handleIncompleteBoot(false, isTimeout, isSSHConnectionRefusedError)
		if failed {
			return s.recordActionFailure(infrav1.ProvisioningError, err.Error())
		}
		if err != nil {
			return actionError{err: fmt.Errorf(errMsgFailedHandlingIncompleteBoot, err)}
		}
		return actionContinue{delay: 10 * time.Second}
	}

	// Check the status of cloud init
	actResult, _ := s.checkCloudInitStatus(sshClient)
	if _, complete := actResult.(actionComplete); !complete {
		return actResult
	}

	// Check whether cloud init did not run successfully even though it shows "done"
	// Check this only when the port did not change. Because if it did, then we can already confirm at this point
	// that the change worked and the new port is usable. This is a strong enough indication for us to assume cloud init worked.
	if s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage == s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit {
		actResult = s.handleCloudInitNotStarted()
		if _, complete := actResult.(actionComplete); !complete {
			return actResult
		}
	}

	record.Event(s.scope.HetznerBareMetalHost, "ServerProvisioned", "server successfully provisioned")
	conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)
	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

// handleConnectionRefused checks cloud init status via ssh to the old ssh port if the new ssh port
// gave a connection refused error.
func (s *Service) handleConnectionRefused() actionResult {
	// Nothing to do if ports didn't change.
	if s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage == s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit {
		return nil
	}
	oldSSHClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})
	actResult, err := s.checkCloudInitStatus(oldSSHClient)
	// If this ssh client also gives an error, then we go back to analyzing the error of the first ssh call
	// This happens in the statement below this one.
	if err == nil {
		// If cloud-init status == "done" and cloud init was successful,
		// then we will soon reboot and be able to access the server via the new port
		if _, complete := actResult.(actionComplete); complete {
			// Check whether cloud init did not run successfully even though it shows "done"
			actResult := s.handleCloudInitNotStarted()
			if _, complete := actResult.(actionComplete); complete {
				return actionContinue{delay: 10 * time.Second}
			}
			return actResult
		}
	}
	if _, actionerr := actResult.(actionError); !actionerr {
		return actResult
	}
	return nil
}

func (s *Service) checkCloudInitStatus(sshClient sshclient.Client) (actionResult, error) {
	out := sshClient.CloudInitStatus()
	// This error is interesting for further logic and might happen because of the fact that the sshClient has the wrong port
	if out.Err != nil {
		return actionError{err: fmt.Errorf("failed to get cloud init status: %w", out.Err)}, out.Err
	}

	stdOut := trimLineBreak(out.StdOut)
	switch {
	case strings.Contains(stdOut, "status: running"):
		// Cloud init is still running
		return actionContinue{delay: 5 * time.Second}, nil
	case strings.Contains(stdOut, "status: disabled"):
		// Reboot needs to be triggered again - did not start yet
		out = sshClient.Reboot()
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to reboot: %w", err)}, nil
		}
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTriggered, "ssh reboot just triggered")
		record.Event(s.scope.HetznerBareMetalHost, "SSHRebootAfterCloudInitStatusDisabled", "cloud-init status was 'disabled'")
		return actionContinue{delay: 5 * time.Second}, nil
	case strings.Contains(stdOut, "status: done"):

		s.scope.HetznerBareMetalHost.ClearError()
	case strings.Contains(stdOut, "status: error"):
		record.Warn(s.scope.HetznerBareMetalHost, "CloudInitFailed", "cloud init returned status error")
		return s.recordActionFailure(infrav1.FatalError, "cloud init returned status error"), nil
	default:
		// Errors are handled after stdOut in this case, as status: error returns an exited with status 1 error
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to get cloud init status: %w", err)}, nil
		}
	}
	return actionComplete{}, nil
}

func (s *Service) handleCloudInitNotStarted() actionResult {
	// Check whether cloud init really was successfully. Sigterm causes problems there.
	oldSSHClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})
	out := oldSSHClient.CheckCloudInitLogsForSigTerm()
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to CheckCloudInitLogsForSigTerm: %w", err)}
	}

	if trimLineBreak(out.StdOut) != "" {
		// it was not succesfull. Prepare and reboot again
		out = oldSSHClient.CleanCloudInitLogs()
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to CleanCloudInitLogs: %w", err)}
		}
		out = oldSSHClient.CleanCloudInitInstances()
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to CleanCloudInitInstances: %w", err)}
		}
		out = oldSSHClient.Reboot()
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to reboot: %w", err)}
		}
		record.Eventf(s.scope.HetznerBareMetalHost,
			"SSHRebootAfterCloudInitSigTermFound", "rebooted via ssh after cloud init logs contained sigterm: %s", trimLineBreak(out.StdOut))
		return actionContinue{delay: 10 * time.Second}
	}

	return actionComplete{}
}

func analyzeSSHOutputProvisioned(out sshclient.Output) (isTimeout, isConnectionRefused bool, reterr error) {
	// check err
	if out.Err != nil {
		switch {
		case os.IsTimeout(out.Err) || sshclient.IsTimeoutError(out.Err):
			isTimeout = true
		case sshclient.IsAuthenticationFailedError(out.Err):
			// As the same ssh key has been used before and after, something is wrong here.
			reterr = fmt.Errorf("wrong ssh key: %w", out.Err)
		case sshclient.IsConnectionRefusedError(out.Err):
			// We strongly assume that the ssh reboot that has been done before has been triggered. Hence we do nothing specific here.
			isConnectionRefused = true
		default:
			reterr = fmt.Errorf("unhandled ssh error while getting hostname: %w", out.Err)
		}
		return isTimeout, isConnectionRefused, reterr
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		return false, false, fmt.Errorf("%w: StdErr: %s", errSSHGetHostname, out.StdErr)
	}

	// check stdout
	switch trimLineBreak(out.StdOut) {
	case "":
		// Hostname should not be empty. This is unexpected.
		return false, false, errEmptyHostName
	case rescue: // We are in wrong boot, nothing has to be done to trigger reboot
		return false, false, nil
	}

	return false, false, fmt.Errorf("%w: %s", errUnexpectedHostName, trimLineBreak(out.StdOut))
}

func (s *Service) actionProvisioned() actionResult {
	// set host to provisioned
	conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)

	rebootDesired := s.scope.HetznerBareMetalHost.HasRebootAnnotation()
	isRebooted := s.scope.HetznerBareMetalHost.Spec.Status.Rebooted
	creds := sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	if rebootDesired {
		if isRebooted {
			// Reboot has been done already. Check whether it has been successful
			// Check hostname with sshClient
			out := sshClient.GetHostName()

			wantHostName := s.scope.Hostname()

			if trimLineBreak(out.StdOut) == wantHostName {
				// Reboot has been successful
				s.scope.HetznerBareMetalHost.Spec.Status.Rebooted = false
				s.scope.HetznerBareMetalHost.ClearRebootAnnotations()

				s.scope.HetznerBareMetalHost.ClearError()
				return actionComplete{}
			}
			// Reboot has been ongoing
			isTimeout, isSSHConnectionRefusedError, err := analyzeSSHOutputProvisioned(out)
			if err != nil {
				if errors.Is(err, errUnexpectedHostName) {
					// One possible reason: The machine gets used by a second wl-cluster
					record.Warnf(s.scope.HetznerBareMetalHost, "UnexpectedHostName",
						"Provisioned: wanted %q. %s", wantHostName, err.Error())
				}
				return actionError{err: fmt.Errorf("failed to handle incomplete boot - provisioning: %w", err)}
			}
			failed, err := s.handleIncompleteBoot(false, isTimeout, isSSHConnectionRefusedError)
			if failed {
				return s.recordActionFailure(infrav1.ProvisioningError, err.Error())
			}
			if err != nil {
				return actionError{err: fmt.Errorf(errMsgFailedHandlingIncompleteBoot, err)}
			}
			return actionContinue{delay: 10 * time.Second}
		}
		// Reboot now
		out := sshClient.Reboot()
		if err := handleSSHError(out); err != nil {
			return actionError{err: err}
		}

		createSSHRebootEvent(s.scope.HetznerBareMetalHost, "reboot annotation was set")
		s.scope.HetznerBareMetalHost.Spec.Status.Rebooted = true
		return actionContinue{delay: 10 * time.Second}
	}

	return actionComplete{}
}

func (s *Service) actionDeprovisioning() actionResult {
	// Update name in robot API
	if _, err := s.scope.RobotClient.SetBMServerName(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name,
	); err != nil {
		s.handleRobotRateLimitExceeded(err, "SetBMServerName")
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	// If has been provisioned completely, stop all running pods
	if s.scope.OSSSHSecret != nil {
		sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
			PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
			Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit,
			IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
		})
		out := sshClient.ResetKubeadm()
		s.scope.V(1).Info("Output of ResetKubeadm", "stdout", out.StdOut, "stderr", out.StdErr, "err", out.Err)
		if out.Err != nil {
			record.Warnf(s.scope.HetznerBareMetalHost, "FailedResetKubeAdm", "failed to reset kubeadm: %s", out.Err.Error())
		} else {
			record.Event(s.scope.HetznerBareMetalHost, "SuccessfulResetKubeAdm", "Reset was successful.")
		}
	} else {
		s.scope.Info("OS SSH Secret is empty - cannot reset kubeadm")
	}

	// Only keep permanent errors on the host object after deprovisioning.
	// Permanent errors are those ones that do not get solved with de- or re-provisioning.
	if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType != infrav1.PermanentError {
		s.scope.HetznerBareMetalHost.ClearError()
	}

	conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)

	return actionComplete{}
}

func (s *Service) actionDeleting() actionResult {
	s.scope.HetznerBareMetalHost.Finalizers = utils.FilterStringFromList(s.scope.HetznerBareMetalHost.Finalizers, infrav1.BareMetalHostFinalizer)
	return deleteComplete{}
}

func (s *Service) handleRobotRateLimitExceeded(err error, functionName string) {
	if models.IsError(err, models.ErrorCodeRateLimitExceeded) || strings.Contains(err.Error(), "server responded with status code 403") {
		msg := fmt.Sprintf("exceeded robot rate limit with calling function %q: %s", functionName, err.Error())
		conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.HetznerAPIReachableCondition,
			infrav1.RateLimitExceededReason,
			clusterv1.ConditionSeverityWarning,
			msg,
		)
		record.Warnf(s.scope.HetznerBareMetalHost, "RateLimitExceeded", msg)
	}
}

// hasJustRebooted returns true if a reboot was done during the last seconds.
// The method gets used to let the controller wait until the reboot was actually done.
// Imagine the controller triggers a reboot, and reconciles immediately. This would
// mean the controller would do the same reboot immediately again.
func (s *Service) hasJustRebooted() bool {
	return (s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeSSHRebootTriggered ||
		s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeSoftwareRebootTriggered ||
		s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeHardwareRebootTriggered) &&
		!hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, rebootWaitTime)
}

func markProvisionPending(host *infrav1.HetznerBareMetalHost, state infrav1.ProvisioningState) {
	conditions.MarkFalse(
		host,
		infrav1.ProvisionSucceededCondition,
		infrav1.StillProvisioningReason,
		clusterv1.ConditionSeverityInfo,
		"host is still provisioning - state %q", state,
	)
}

func createSSHRebootEvent(host *infrav1.HetznerBareMetalHost, msg string) {
	createRebootEvent(host, infrav1.RebootTypeSSH, msg)
}

func createRebootEvent(host *infrav1.HetznerBareMetalHost, rebootType infrav1.RebootType, msg string) string {
	verboseRebootType := infrav1.VerboseRebootType(rebootType)
	reason := fmt.Sprintf("RebootBMServerVia%sProvisioningState%s",
		verboseRebootType,
		strcase.UpperCamelCase(string(host.Spec.Status.ProvisioningState)))
	msg = fmt.Sprintf("Phase %s, reboot via %s: %s", host.Spec.Status.ProvisioningState, verboseRebootType, msg)
	record.Eventf(host, reason, msg)
	return msg
}
