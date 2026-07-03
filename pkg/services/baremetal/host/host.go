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
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stoewer/go-strcase"
	"github.com/syself/hrobot-go/models"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

const (
	rebootWaitTime           time.Duration = 15 * time.Second
	sshResetTimeout          time.Duration = 8 * time.Minute
	softwareResetTimeout     time.Duration = 10 * time.Minute
	hardwareResetTimeout     time.Duration = 10 * time.Minute
	connectionRefusedTimeout time.Duration = 10 * time.Minute
	rescue                   string        = "rescue"
	rescuePort               int           = 22
	gbToMebiBytes            int           = 1000
	gbToBytes                int           = 1000000 * gbToMebiBytes
	kikiToMebiBytes          int           = 1024

	errMsgFailedReboot                 = "failed to reboot bare metal server: %w"
	errMsgInvalidSSHStdOut             = "invalid output in stdOut: %w"
	errMsgFailedHandlingIncompleteBoot = "failed to handle incomplete boot: %w"
	rebootServerStr                    = "RebootBMServer"

	// PostInstallScriptFinished is a marker in the output of installimage. If it is not present,
	// then install-image failed.
	PostInstallScriptFinished = "POST_INSTALL_SCRIPT_FINISHED"
)

var (
	baremetalImageURLCommandDir = "/shared"

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

	if !s.scope.HetznerBareMetalHost.DeletionTimestamp.IsZero() {
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.HostReadyCondition,
			infrav1.DeletionInProgressReason,
			clusterv1beta1.ConditionSeverityWarning,
			"Host is not ready because it is being deleted",
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostDeletingV1Beta2Condition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav1.HetznerBareMetalHostDeletingV1Beta2Reason,
			Message: "Host is being deleted",
		})
	}

	hostStateMachine := newHostStateMachine(s.scope.HetznerBareMetalHost, s, s.scope.Logger)

	// reconcile state
	actResult := hostStateMachine.ReconcileState(ctx)

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

// previous: None
// next: Registering
func (s *Service) actionPreparing(ctx context.Context) actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StatePreparing)

	server, err := s.scope.RobotClient.GetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		// If Robot API returned "unauthorized" error - mark condition RobotCredentialsAvailable as false
		// with reason RobotCredentialsInvalid and stop reconciling.
		if models.IsError(err, models.ErrorCodeUnauthorized) {
			msg := "Robot API returned unauthorized; verify the credentials in the referenced secret are correct"
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.RobotCredentialsAvailableCondition,
				infrav1.RobotCredentialsInvalidReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostRobotCredentialsInvalidV1Beta2Reason,
				Message: msg,
			})
			record.Warnf(s.scope.HetznerBareMetalHost, infrav1.RobotCredentialsInvalidReason, msg)

			return actionStop{}
		}

		s.handleRobotRateLimitExceeded(err, "GetBMServer")
		if models.IsError(err, models.ErrorCodeServerNotFound) {
			msg := "GetBMServer (Robot API) replied: ServerNotFound"
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.ServerNotFoundReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostServerNotFoundV1Beta2Reason,
				Message: msg,
			})
			record.Warnf(s.scope.HetznerBareMetalHost, infrav1.ServerNotFoundReason, msg)
			s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
			return actionStop{}
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			// If the Hetzner API returns this, we just want to retry later:
			// Get "https://robot-ws.your-server.de/server/1234": net/http: TLS handshake timeout
			s.scope.Info("GetBMServer timed out, will retry later", "error", err)
			return actionContinue{
				delay: 10 * time.Second,
			}
		}
		return actionError{err: fmt.Errorf("failed to get bare metal server: %w", err)}
	}

	v1beta1conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RobotCredentialsAvailableCondition)
	v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
		Type:   infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason,
	})

	if server.ServerIP == "" {
		msg := fmt.Sprintf("bare metal server %d has no IPv4 address assigned", s.scope.HetznerBareMetalHost.Spec.ServerID)
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.ServerHasNoIPv4Reason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			msg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostServerHasNoIPv4V1Beta2Reason,
			Message: msg,
		})
		record.Warnf(s.scope.HetznerBareMetalHost, infrav1.ServerHasNoIPv4Reason, msg)
		s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
		return actionStop{}
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

	if err := s.enforceRescueMode(); err != nil {
		return actionError{err: fmt.Errorf("failed to enforce rescue mode: %w", err)}
	}

	if s.scope.SSHAfterInstallImageEnabled() {
		// We have ssh access to running nodes. Maybe we can reboot via ssh instead of
		// using the robot API.
		sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
			PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
			Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
			IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
		})

		// Check hostname with sshClient
		out := sshClient.GetHostName(ctx)
		if trimLineBreak(out.StdOut) != "" {
			// we managed access with ssh - we can do an ssh reboot
			if err := handleSSHError(sshClient.Reboot(ctx)); err != nil {
				return actionError{err: fmt.Errorf("failed to reboot server via ssh (actionPreparing): %w", err)}
			}
			msg := "Rebooting into rescue mode."
			createSSHRebootEvent(ctx, s.scope.HetznerBareMetalHost, msg)
			s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
			// we immediately set an error message in the host status to track the reboot we just performed
			s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTriggered, fmt.Sprintf("Phase %s, reboot via ssh: %s",
				s.scope.HetznerBareMetalHost.Spec.Status.ProvisioningState, msg))
			return actionComplete{} // next: Registering
		}
	}

	// Check if software reboot is available. If it is not, choose hardware reboot.
	rebootType, errorType := rebootAndErrorTypeAfterTimeout(s.scope.HetznerBareMetalHost)

	if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
		s.handleRobotRateLimitExceeded(err, rebootServerStr)
		return actionError{err: fmt.Errorf(errMsgFailedReboot, err)}
	}

	s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
	msg := createRebootEvent(ctx, s.scope.HetznerBareMetalHost, rebootType, "Reboot into rescue system.")
	// we immediately set an error message in the host status to track the reboot we just performed.
	// This is not a real error. Sooner or later we should track the reboots differently.
	s.scope.HetznerBareMetalHost.SetError(errorType, msg)
	return actionComplete{} // next: Registering
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

// ensureSSHKey ensures that the given ssh key is known to the Robot-API.
// s.scope.RobotClient.SetSSHKey() gets used to upload the public-key, if it is not there yet.
func (s *Service) ensureSSHKey(sshSecretRef infrav1.SSHSecretRef, sshSecret *corev1.Secret) (infrav1.SSHKey, actionResult) {
	if sshSecret == nil {
		return infrav1.SSHKey{}, actionError{err: errNilSSHSecret}
	}
	hetznerSSHKeys, err := s.scope.RobotClient.ListSSHKeys()
	if err != nil {
		s.handleRobotRateLimitExceeded(err, "ListSSHKeys")
		if !models.IsError(err, models.ErrorCodeNotFound) {
			return infrav1.SSHKey{}, actionError{err: fmt.Errorf("failed to list ssh keys: %w", err)}
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
				// Robot SSH-keys API is a bit strange: the public key value must be unique; you
				// can’t add the same key twice. Uniqueness is checked by key value, not by the
				// key’s name.
				msg := fmt.Sprintf("cannot upload ssh key %q (from secret %q) - exists already under a different name: %s",
					string(sshSecret.Data[sshSecretRef.Key.Name]), sshSecretRef.Name, err.Error())
				v1beta1conditions.MarkFalse(
					s.scope.HetznerBareMetalHost,
					infrav1.CredentialsAvailableCondition,
					infrav1.SSHKeyAlreadyExistsReason,
					clusterv1beta1.ConditionSeverityError,
					"%s",
					msg,
				)
				v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
					Type:    infrav1.HetznerBareMetalHostSSHKeysAvailableV1Beta2Condition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav1.HetznerBareMetalHostSSHKeyAlreadyExistsV1Beta2Reason,
					Message: msg,
				})
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

// handleIncompleteBoot checks if the reboot was successful.
// If it was not successful, it tries other reboot methods.
// Order: SSH -> Software -> Hardware.
func (s *Service) handleIncompleteBoot(ctx context.Context, isRebootIntoRescue, isTimeout, isConnectionRefused bool) (failed bool, err error) {
	// Connection refused error might be a sign that the ssh port is wrong - but might also come
	// right after a reboot and is expected then. Therefore, we wait for some time and if the
	// error keeps coming, we give an error.
	if isConnectionRefused {
		if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
			// if error has occurred before, check the timeout
			if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt, connectionRefusedTimeout) {
				msg := "Connection error when targeting server with ssh that might be due to a wrong ssh port. Please check."
				if isRebootIntoRescue {
					msg = "Connection error. Can't reach rescue system via ssh."
				}
				v1beta1conditions.MarkFalse(
					s.scope.HetznerBareMetalHost,
					infrav1.ProvisionSucceededCondition,
					infrav1.SSHConnectionRefusedReason,
					clusterv1beta1.ConditionSeverityError,
					"%s",
					msg,
				)
				v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
					Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav1.HetznerBareMetalHostSSHConnectionRefusedV1Beta2Reason,
					Message: msg,
				})
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
			// As the server has no error set yet, set error message and return.
			s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTriggered, "ssh timeout error - server has not restarted yet")
			return false, nil
		}

		// We did not get an error with ssh - but also not the expected hostname. Therefore,
		// the (ssh) reboot did not start. We trigger an API reboot instead.
		return false, s.handleErrorTypeSSHRebootFailed(ctx, isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeSSHRebootTriggered:
		return false, s.handleErrorTypeSSHRebootFailed(ctx, isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeSoftwareRebootTriggered:
		return false, s.handleErrorTypeSoftwareRebootFailed(ctx, isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeHardwareRebootTriggered:
		return s.handleErrorTypeHardwareRebootFailed(ctx, isTimeout, isRebootIntoRescue)
	}

	return false, fmt.Errorf("%w: %s", errUnexpectedErrorType, s.scope.HetznerBareMetalHost.Spec.Status.ErrorType)
}

func (s *Service) handleErrorTypeSSHRebootFailed(ctx context.Context, isSSHTimeoutError, wantsRescue bool) error {
	// If it is not a timeout error, then the ssh command (get hostname) worked, but didn't give us the
	// right hostname. This means that the server has not been rebooted and we need to escalate.
	// If we got a timeout error from ssh, it means that the server has not yet finished rebooting.
	// If the timeout for ssh reboots has been reached, then escalate.
	rebootInto := "node"
	if wantsRescue {
		rebootInto = "rescue mode"
	}
	if !isSSHTimeoutError || hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt, sshResetTimeout) {
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
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
		msg := fmt.Sprintf("Reboot via ssh into %s failed. Now using rebootType %q.",
			rebootInto, rebootType)
		msg = createRebootEvent(ctx, s.scope.HetznerBareMetalHost, rebootType, msg)
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

func (s *Service) handleErrorTypeSoftwareRebootFailed(ctx context.Context, isSSHTimeoutError, wantsRescue bool) error {
	rebootInto := "node"
	if wantsRescue {
		rebootInto = "rescue mode"
	}
	// If it is not a timeout error, then the ssh command (get hostname) worked, but didn't give us the
	// right hostname. This means that the server has not been rebooted and we need to escalate.
	// If we got a timeout error from ssh, it means that the server has not yet finished rebooting.
	// If the timeout for software reboots has been reached, then escalate.
	if !isSSHTimeoutError || hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt, softwareResetTimeout) {
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
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
		msg := fmt.Sprintf("Reboot via type 'software' into %s failed. Now using rebootType %q.",
			rebootInto, infrav1.RebootTypeHardware)
		msg = createRebootEvent(ctx, s.scope.HetznerBareMetalHost, infrav1.RebootTypeHardware, msg)
		// we immediately set an error message in the host status to track the reboot we just performed
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootTriggered, msg)
	}

	return nil
}

// handleErrorTypeHardwareRebootFailed deals with hardware reboot failed cases and returns whether we should fail the process.
func (s *Service) handleErrorTypeHardwareRebootFailed(ctx context.Context, isSSHTimeoutError, wantsRescue bool) (bool, error) {
	rebootInto := "node"
	if wantsRescue {
		rebootInto = "rescue mode"
	}
	// If it is not a timeout error, then the ssh command (get hostname) worked, but didn't give us the
	// right hostname. This means that the server has not been rebooted and we need to escalate.
	// If we got a timeout error from ssh, it means that the server has not yet finished rebooting.
	// If the timeout for hardware reboots has been reached, then escalate.
	if !isSSHTimeoutError {
		if wantsRescue {
			// make sure hat we boot into rescue mode if that is necessary
			if err := s.ensureRescueMode(); err != nil {
				return false, fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}

		// we immediately set an error message in the host status to track the reboot we just performed
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			s.handleRobotRateLimitExceeded(err, rebootServerStr)
			return false, fmt.Errorf(errMsgFailedReboot, err)
		}
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
		msg := fmt.Sprintf("Reboot via ssh into %s failed. Now using rebootType %q.",
			rebootInto, infrav1.RebootTypeHardware)
		createRebootEvent(ctx, s.scope.HetznerBareMetalHost, infrav1.RebootTypeHardware, msg)
	}

	// if hardware reboots time out, we should fail
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt, hardwareResetTimeout) {
		msg := "reboot to node timed out - please check if server is working properly"
		if wantsRescue {
			msg = "The rescue system could not be reached. Please ensure that the machine tries to boot from network before booting from disk. This setting needs to be enabled permanently in the BIOS."
		}
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.RebootTimedOutReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			msg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostRebootTimeoutReachedV1Beta2Reason,
			Message: msg,
		})

		record.Warn(s.scope.HetznerBareMetalHost, "HardwareRebootTimedOut", msg)

		return true, fmt.Errorf("hardware reboot (to %s) timed out", rebootInto)
	}

	return false, nil
}

func hasTimedOut(lastUpdated *metav1.Time, timeout time.Duration) bool {
	if lastUpdated != nil {
		now := metav1.Now()
		return lastUpdated.Add(timeout).Before(now.Time)
	}

	return false
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

// previous: Preparing
// next: PreProvisioning
func (s *Service) actionRegistering(ctx context.Context) actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateRegistering)

	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	// Check hostname with sshClient
	out := sshClient.GetHostName(ctx)
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

		failed, err := s.handleIncompleteBoot(ctx, true, isSSHTimeoutError, isSSHConnectionRefusedError)
		if failed {
			return s.recordActionFailure(infrav1.PermanentError, err.Error())
		}
		if err != nil {
			return actionError{err: fmt.Errorf(errMsgFailedHandlingIncompleteBoot, err)}
		}

		timeSinceReboot := "unknown"
		if s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt != nil {
			timeSinceReboot = time.Since(s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt.Time).Round(time.Second).String()
		}

		s.scope.Info("Could not reach rescue system. Will retry some seconds later.", "out", out.String(), "hostName", hostName,
			"isSSHTimeoutError", isSSHTimeoutError, "isSSHConnectionRefusedError", isSSHConnectionRefusedError, "timeSinceReboot", timeSinceReboot)
		return actionContinue{delay: 10 * time.Second}
	}

	// we are in rescue mode i.e. reboot was successful, now clear the RebootTriggeredAt timestamp.
	s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = nil

	output := sshClient.GetHardwareDetailsDebug(ctx)
	if output.Err != nil {
		return actionError{err: fmt.Errorf("failed to obtain hardware for debugging: %w", output.Err)}
	}

	msg := fmt.Sprintf("%s\n\n", output.StdOut)
	if output.StdErr != "" {
		msg += fmt.Sprintf("stderr:\n%s\n\n", output.StdErr)
	}
	record.Eventf(s.scope.HetznerBareMetalHost, "GetHardwareDetails", msg)

	if s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails == nil {
		hardwareDetails, err := getHardwareDetails(ctx, sshClient)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to get hardware details: %w", err)}
		}
		s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails = &hardwareDetails
	}

	if s.scope.HetznerBareMetalHost.Spec.RootDeviceHints == nil {
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1beta1.ConditionSeverityError,
			infrav1.ErrorMessageMissingRootDeviceHints,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason,
			Message: infrav1.ErrorMessageMissingRootDeviceHints,
		})
		record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason, infrav1.ErrorMessageMissingRootDeviceHints)
		return s.recordActionFailure(infrav1.RegistrationError, infrav1.ErrorMessageMissingRootDeviceHints)
	}
	errMsg := s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.IsValidWithMessage()
	if errMsg != "" {
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			errMsg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason,
			Message: errMsg,
		})
		record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason, errMsg)
		return s.recordActionFailure(infrav1.RegistrationError, errMsg)
	}

	if err := validateRootDeviceWwnsAreSubsetOfExistingWwns(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints,
		s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails.Storage); err != nil {
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			err.Error(),
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason,
			Message: err.Error(),
		})
		record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason, err.Error())
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
		s.scope.Info(msg)
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.RootDeviceHintsValidatedCondition,
			infrav1.ValidationFailedReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			msg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason,
			Message: msg,
		})
		record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostValidationFailedV1Beta2Reason, msg)
		return s.recordActionFailure(infrav1.FatalError, msg)
	}

	v1beta1conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RootDeviceHintsValidatedCondition)
	v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
		Type:   infrav1.HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HetznerBareMetalHostRootDeviceHintsValidatedV1Beta2Reason,
	})
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

func getHardwareDetails(ctx context.Context, sshClient sshclient.Client) (infrav1.HardwareDetails, error) {
	mebiBytes, err := obtainHardwareDetailsRAM(ctx, sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details RAM: %w", err)
	}

	nics, err := obtainHardwareDetailsNics(ctx, sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details Nics: %w", err)
	}

	storage, err := obtainHardwareDetailsStorage(ctx, sshClient)
	if err != nil {
		return infrav1.HardwareDetails{}, fmt.Errorf("failed to obtain hardware details storage: %w", err)
	}

	// remove names of storage devices because they might change
	for i := range storage {
		storage[i].Name = ""
	}

	cpu, err := obtainHardwareDetailsCPU(ctx, sshClient)
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
	switch {
	case os.IsTimeout(sshErr) || sshclient.IsTimeoutError(sshErr):
		isSSHTimeoutError = true
	case sshclient.IsAuthenticationFailedError(sshErr):
		// check if the reboot triggered
		rebootTriggered, err := s.rebootTriggered()
		if err != nil {
			return false, false, fmt.Errorf("failed to check whether reboot triggered: %w", err)
		}

		if !rebootTriggered {
			return false, false, nil
		}
		reterr = fmt.Errorf("wrong ssh key: %w", sshErr)
	case sshclient.IsConnectionRefusedError(sshErr):
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

func obtainHardwareDetailsRAM(ctx context.Context, sshClient sshclient.Client) (int, error) {
	out := sshClient.GetHardwareDetailsRAM(ctx)
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

func obtainHardwareDetailsNics(ctx context.Context, sshClient sshclient.Client) ([]infrav1.NIC, error) {
	type originalNic struct {
		Name      string `json:"name,omitempty"`
		Model     string `json:"model,omitempty"`
		MAC       string `json:"mac,omitempty"`
		IP        string `json:"ip,omitempty"`
		SpeedMbps string `json:"speedMbps,omitempty"`
	}

	out := sshClient.GetHardwareDetailsNics(ctx)
	if err := handleSSHError(out); err != nil {
		return nil, err
	}
	stdOut := trimLineBreak(out.StdOut)
	if stdOut == "" {
		return nil, sshclient.ErrEmptyStdOut
	}

	stringArray := strings.Split(stdOut, "\n")
	nicsArray := make([]infrav1.NIC, 0, len(stringArray))

	ipFound := false
	for _, str := range stringArray {
		validJSONString := validJSONFromSSHOutput(str)

		var nic originalNic
		if err := json.Unmarshal([]byte(validJSONString), &nic); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %v. Original ssh output %s: %w", validJSONString, stdOut, err)
		}

		// speedMbps can be empty
		if nic.SpeedMbps == "" {
			nic.SpeedMbps = "0"
		}
		speedMbps, err := strconv.Atoi(nic.SpeedMbps)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int from string %s: %w", nic.SpeedMbps, err)
		}

		nicsArray = append(nicsArray, infrav1.NIC{
			Name:      nic.Name,
			Model:     nic.Model,
			MAC:       nic.MAC,
			IP:        nic.IP,
			SpeedMbps: speedMbps,
		})

		if nic.IP != "" {
			ipFound = true
		}
	}
	// if no IP was found, we return an error
	// See nodeAddresses()
	if !ipFound {
		return nil, fmt.Errorf("no IP found in NICs: %+v", nicsArray)
	}

	return nicsArray, nil
}

func obtainHardwareDetailsStorage(ctx context.Context, sshClient sshclient.Client) ([]infrav1.Storage, error) {
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

	out := sshClient.GetHardwareDetailsStorage(ctx)
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

func obtainHardwareDetailsCPU(ctx context.Context, sshClient sshclient.Client) (cpu infrav1.CPU, err error) {
	cpu.Arch, err = getCPUArch(ctx, sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU arch: %w", err)
	}

	cpu.Model, err = getCPUModel(ctx, sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU model: %w", err)
	}

	cpu.ClockGigahertz, err = getCPUClockGigahertz(ctx, sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU clock speed: %w", err)
	}

	cpu.Threads, err = getCPUThreads(ctx, sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU threads: %w", err)
	}

	cpu.Flags, err = getCPUFlags(ctx, sshClient)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to get CPU flags: %w", err)
	}

	return cpu, nil
}

func getCPUArch(ctx context.Context, sshClient sshclient.Client) (string, error) {
	out := sshClient.GetHardwareDetailsCPUArch(ctx)
	if err := handleSSHError(out); err != nil {
		return "", err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return "", fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}

	return stdOut, nil
}

func getCPUModel(ctx context.Context, sshClient sshclient.Client) (string, error) {
	out := sshClient.GetHardwareDetailsCPUModel(ctx)
	if err := handleSSHError(out); err != nil {
		return "", err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return "", fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}
	return stdOut, nil
}

func getCPUClockGigahertz(ctx context.Context, sshClient sshclient.Client) (infrav1.ClockSpeed, error) {
	out := sshClient.GetHardwareDetailsCPUClockGigahertz(ctx)
	if err := handleSSHError(out); err != nil {
		return infrav1.ClockSpeed(""), err
	}

	stdOut, err := validateStdOut(out.StdOut)
	if err != nil {
		return infrav1.ClockSpeed(""), fmt.Errorf(errMsgInvalidSSHStdOut, err)
	}

	return infrav1.ClockSpeed(stdOut), nil
}

func getCPUThreads(ctx context.Context, sshClient sshclient.Client) (int, error) {
	out := sshClient.GetHardwareDetailsCPUThreads(ctx)
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

func getCPUFlags(ctx context.Context, sshClient sshclient.Client) ([]string, error) {
	out := sshClient.GetHardwareDetailsCPUFlags(ctx)
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

// previous: Registering
// next: ImageInstalling
func (s *Service) actionPreProvisioning(ctx context.Context) actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StatePreProvisioning)

	// Ensure os ssh secret
	sshKey, actResult := s.ensureSSHKey(s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef, s.scope.OSSSHSecret)
	if _, isComplete := actResult.(actionComplete); !isComplete {
		return actResult
	}
	s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.OSKey = &sshKey

	if s.scope.PreProvisionCommand == "" {
		return actionComplete{}
	}

	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	out := sshClient.GetHostName(ctx)
	if out.Err != nil || out.StdErr != "" {
		ctrl.LoggerFrom(ctx).Info("pre-provision: rescue system not reachable. Will try again",
			"sshOutput", out.String())
		return actionContinue{delay: 10 * time.Second}
	}

	hostName := trimLineBreak(out.StdOut)
	if hostName != rescue {
		// This is unexpected. We should be in rescue mode.
		msg := fmt.Sprintf("expected rescue system, but found different hostname %q", hostName)
		record.Warnf(s.scope.HetznerBareMetalHost, "PreProvisioningFailed", msg)
		ctrl.LoggerFrom(ctx).Error(errors.New("PreProvisioningFailed"), msg)
		s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
		return actionStop{}
	}

	exitStatus, output, err := sshClient.ExecutePreProvisionCommand(ctx, s.scope.PreProvisionCommand)
	if err != nil {
		return actionError{err: fmt.Errorf("failed to execute pre-provision command: %w", err)}
	}

	if exitStatus != 0 {
		record.Warnf(s.scope.HetznerBareMetalHost, "PreProvisionCommandFailed",
			"%s: %s", filepath.Base(s.scope.PreProvisionCommand), output)
		s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, output)
		return actionStop{}
	}

	record.Eventf(s.scope.HetznerBareMetalHost, "PreProvisionCommandSucceeded",
		"%s: %s", filepath.Base(s.scope.PreProvisionCommand), output)

	return actionComplete{}
}

// previous: PreProvisioning
// next: EnsureProvisioned
func (s *Service) actionImageInstalling(ctx context.Context) actionResult {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateImageInstalling)

	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	out := sshClient.GetHostName(ctx)
	if out.Err != nil || out.StdErr != "" {
		ctrl.LoggerFrom(ctx).Info("image-installing: rescue system not reachable. Will try again",
			"sshOutput", out.String())
		return actionContinue{delay: 10 * time.Second}
	}

	hostName := trimLineBreak(out.StdOut)
	realHostName := s.scope.Hostname()
	if hostName != rescue && hostName != realHostName {
		// During InstallImage the hostname changes from "rescue" to the realHostName.
		// If it is not one of these two, then this is unexpected.
		// This is unexpected. We should be in rescue mode.
		msg := fmt.Sprintf("expected rescue system (%q or %q), but found different hostname %q",
			rescue, realHostName, hostName)
		record.Warnf(s.scope.HetznerBareMetalHost, "ImageInstallingFailed", msg)
		ctrl.LoggerFrom(ctx).Error(errors.New("ImageInstallingFailed"), msg)
		s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
		return actionStop{}
	}

	if s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.UsesImageURLCommand() {
		return s.actionImageInstallingImageURLCommand(ctx, sshClient)
	}
	state, err := sshClient.GetInstallImageState(ctx)
	if err != nil {
		return actionError{err: fmt.Errorf("failed to get state of installimage processes: %w", err)}
	}

	switch state {
	case sshclient.InstallImageStateRunning:
		s.scope.Info("installimage is still running. Checking again in some seconds.")
		return actionContinue{delay: 10 * time.Second}
	case sshclient.InstallImageStateFinished:
		s.scope.Info("installimage is finished.")
		return s.actionImageInstallingFinished(ctx, sshClient)
	case sshclient.InstallImageStateNotStartedYet:
		s.scope.Info("installimage is not started yet. Starting it now")
		return s.actionImageInstallingStartBackgroundProcess(ctx, sshClient)
	default:
		panic(fmt.Sprintf("Unknown InstallImageState %+v", state))
	}
}

func (s *Service) actionImageInstallingImageURLCommand(ctx context.Context, sshClient sshclient.Client) actionResult {
	host := s.scope.HetznerBareMetalHost

	state, logFile, err := sshClient.StateOfImageURLCommand(ctx)
	if err != nil {
		return actionError{err: fmt.Errorf("StateOfImageURLCommand failed: %w", err)}
	}

	var duration time.Duration
	if host.Spec.Status.RebootTriggeredAt != nil {
		duration = time.Since(host.Spec.Status.RebootTriggeredAt.Time)
	}

	// Please keep the number (20) in sync with the docstring of ImageURL.
	if duration > 20*time.Minute {
		// timeout. Something has failed.
		msg := fmt.Sprintf("ImageURLCommand timed out after %s. Deleting machine",
			duration.Round(time.Second).String())
		s.scope.Error(nil, msg, "logFile", logFile)
		record.Warn(s.scope.HetznerBareMetalHost, "ImageURLCommandTimedOut", logFile)

		v1beta1conditions.MarkFalse(host, infrav1.ProvisionSucceededCondition,
			"ImageURLCommandTimedOut", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  "ImageURLCommandTimedOut",
			Message: msg,
		})
		return s.recordActionFailure(infrav1.FatalError, msg)
	}

	switch state {
	case sshclient.ImageURLCommandStateRunning:
		return actionContinue{delay: 10 * time.Second}

	case sshclient.ImageURLCommandStateFinishedSuccessfully:
		record.Event(s.scope.HetznerBareMetalHost, "ImageURLCommandOutput", logFile)
		s.scope.Info("ImageURLCommandOutput", "logFile", logFile)

		// Update name in robot API
		if _, err := s.scope.RobotClient.SetBMServerName(s.scope.HetznerBareMetalHost.Spec.ServerID, s.scope.Hostname()); err != nil {
			record.Warn(s.scope.HetznerBareMetalHost, "SetBMServerNameFailed", err.Error())
			s.handleRobotRateLimitExceeded(err, "SetBMServerName")
			return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
		}

		// Reboot via SSH
		if err := sshClient.Reboot(ctx).Err; err != nil {
			err = fmt.Errorf("failed to reboot server (after install-image): %w", err)
			record.Warn(s.scope.HetznerBareMetalHost, "RebootFailed", err.Error())
			return actionError{err: err}
		}

		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())

		msg := "machine image and cloud-init data got installed (via image-url-command)"
		createSSHRebootEvent(ctx, s.scope.HetznerBareMetalHost, msg)

		// clear potential errors - all done
		s.scope.HetznerBareMetalHost.ClearError()
		return actionComplete{}

	case sshclient.ImageURLCommandStateFailed:
		record.Warn(s.scope.HetznerBareMetalHost, "InstallImageNotSuccessful", logFile)
		msg := "image-url-command failed"
		s.scope.Error(nil, msg, "logFile", logFile)
		v1beta1conditions.MarkFalse(host, infrav1.ProvisionSucceededCondition,
			"ImageURLCommandFailed", clusterv1beta1.ConditionSeverityWarning,
			"%s", msg)
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  "ImageURLCommandFailed",
			Message: msg,
		})
		return s.recordActionFailure(infrav1.FatalError, msg)

	case sshclient.ImageURLCommandStateNotStarted:
		data, err := s.scope.GetRawBootstrapData(ctx)
		if err != nil {
			return actionError{err: fmt.Errorf("baremetal GetRawBootstrapData failed: %w", err)}
		}

		command := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.ImageURLCommand
		if command == "" {
			err = errors.New("internal error: spec.status.installImage.imageURLCommand is not set")
			s.scope.Error(err, "")
			record.Warn(s.scope.HetznerBareMetalHost, "ImageURLCommandMissing", err.Error())

			v1beta1conditions.MarkFalse(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition,
				"ImageURLCommandMissing",
				clusterv1beta1.ConditionSeverityError,
				"%s", err.Error())
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  "ImageURLCommandMissing",
				Message: err.Error(),
			})
			return actionStop{}
		}

		commandPath, err := utils.ResolveImageURLCommandPath(baremetalImageURLCommandDir, command)
		if err != nil {
			err = fmt.Errorf("imageURLCommand %q is invalid or not accessible by the controller pod: %w", command, err)
			s.scope.Error(err, "")
			record.Warn(s.scope.HetznerBareMetalHost, "ImageURLCommandNotAccessible", err.Error())

			v1beta1conditions.MarkFalse(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition,
				"ImageURLCommandNotAccessible",
				clusterv1beta1.ConditionSeverityWarning,
				"%s", err.Error())
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  "ImageURLCommandNotAccessible",
				Message: err.Error(),
			})
			return actionStop{}
		}

		// get device names from storage device
		var deviceNames []string
		switch s.scope.HetznerBareMetalMachine.Spec.InstallImage.DeviceStringType {
		case infrav1.DeviceStringTypeWWN:
			// WWN examples: "eui.00253885910c8cec" or "0x500a07511bb48b25"
			deviceNames = s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN()
			if len(deviceNames) == 0 {
				// this is not expected, because it is already validated.
				return actionError{err: fmt.Errorf("DeviceStringType is %q but no WWN is configured in rootDeviceHints", infrav1.DeviceStringTypeWWN)}
			}
		default:
			// Short device name examples: "sda", "sdb"
			// Get the information about storage devices again to have the latest names.
			// Device names can change during restart.
			storage, err := obtainHardwareDetailsStorage(ctx, sshClient)
			if err != nil {
				return actionError{err: fmt.Errorf("failed to obtain hardware details storage: %w", err)}
			}
			deviceNames = getDeviceNames(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN(), storage)
		}

		exitStatus, stdoutStderr, err := sshClient.StartImageURLCommand(ctx, commandPath, s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image.URL, data, s.scope.Hostname(), deviceNames)
		if err != nil {
			err := fmt.Errorf("StartImageURLCommand failed (retrying): %w", err)
			// This could be a temporary network error. Retry.
			s.scope.Error(err, "",
				"ImageURLCommand", command,
				"exitStatus", exitStatus,
				"stdoutStderr", stdoutStderr)
			record.Warn(s.scope.HetznerBareMetalHost, "ImageURLCommandFailedToStart", err.Error())

			v1beta1conditions.MarkFalse(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition,
				"ImageURLCommandFailedToStart",
				clusterv1beta1.ConditionSeverityWarning,
				"%s", err.Error())
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  "ImageURLCommandFailedToStart",
				Message: err.Error(),
			})
			return actionError{err: err}
		}

		if exitStatus != 0 {
			msg := "StartImageURLCommand failed with non-zero exit status. Deleting machine"
			s.scope.Error(nil, msg,
				"ImageURLCommand", command,
				"exitStatus", exitStatus,
				"stdoutStderr", stdoutStderr)
			record.Warn(s.scope.HetznerBareMetalHost, "StartImageURLCommandFailed", msg)

			v1beta1conditions.MarkFalse(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition,
				"StartImageURLCommandFailed",
				clusterv1beta1.ConditionSeverityWarning,
				"%s", msg)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  "StartImageURLCommandFailed",
				Message: msg,
			})
			return s.recordActionFailure(infrav1.ProvisioningError, msg)
		}

		v1beta1conditions.MarkFalse(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition,
			"ImageURLCommandStarted",
			clusterv1beta1.ConditionSeverityInfo,
			"imageURLCommand started")
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  "ImageURLCommandStarted",
			Message: "imageURLCommand started",
		})

		return actionContinue{delay: 55 * time.Second}

	default:
		return actionError{err: fmt.Errorf("unknown ImageURLCommandState: %q", state)}
	}
}

func (s *Service) actionImageInstallingStartBackgroundProcess(ctx context.Context, sshClient sshclient.Client) actionResult {
	// CheckDisk before accessing the disk
	info, err := sshClient.CheckDisk(ctx, s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN())
	if err != nil {
		_, annotationOk := s.scope.HetznerBareMetalHost.Annotations[infrav1.IgnoreCheckDiskAnnotation]
		machineSkipsCheckDisk := s.scope.HetznerBareMetalMachine.Spec.SkipCheckDisk
		if !annotationOk && !machineSkipsCheckDisk {
			// Neither the annotation nor the machine spec field is set. This is a permanent error.
			msg := fmt.Sprintf(
				"CheckDisk failed (permanent error): %s (set annotation %q on hbmh or skipCheckDisk on HetznerBareMetalMachine to skip)",
				err.Error(), infrav1.IgnoreCheckDiskAnnotation)
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.CheckDiskFailedReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostCheckingDiskFailedV1Beta2Reason,
				Message: msg,
			})
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.CheckDiskFailedReason, msg)
			s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
			return actionStop{}
		}
		// The annotation or machine spec field was set. Just create a warning and move on.
		record.Warnf(s.scope.HetznerBareMetalHost, infrav1.CheckDiskFailedReason,
			"CheckDisk failed. Skipping because %q is set or skipCheckDisk is true on HetznerBareMetalMachine: %s",
			infrav1.IgnoreCheckDiskAnnotation,
			err.Error())
	} else {
		record.Eventf(s.scope.HetznerBareMetalHost, "DiskHealthy", "Disk looks healthy: %s", info)
	}

	// Call WipeDisk if the corresponding annotation is set.
	sliceOfWwns := strings.Fields(s.scope.HetznerBareMetalHost.Annotations[infrav1.WipeDiskAnnotation])
	if len(sliceOfWwns) > 0 {
		output, err := sshClient.WipeDisk(ctx, sliceOfWwns)
		if err != nil {
			var exitErr *ssh.ExitError
			if errors.As(err, &exitErr) || errors.Is(err, sshclient.ErrInvalidWWN) {
				// The script was executed, but an error occurred.
				// Do not retry. This needs manual intervention.
				msg := fmt.Sprintf("WipeDisk failed (permanent error): %s",
					err.Error())
				v1beta1conditions.MarkFalse(
					s.scope.HetznerBareMetalHost,
					infrav1.ProvisionSucceededCondition,
					infrav1.WipeDiskFailedReason,
					clusterv1beta1.ConditionSeverityError,
					"%s",
					msg,
				)
				v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
					Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav1.HetznerBareMetalHostWipingDiskFailedV1Beta2Reason,
					Message: msg,
				})
				record.Warn(s.scope.HetznerBareMetalHost, infrav1.WipeDiskFailedReason, msg)
				s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
				return actionStop{}
			}
			// some other error happened. It is likely that the ssh connection failed.
			msg := fmt.Sprintf("WipeDisk failed (Will retry): %s",
				err.Error())
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.WipeDiskFailedReason,
				clusterv1beta1.ConditionSeverityWarning,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostWipingDiskFailedV1Beta2Reason,
				Message: msg,
			})
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.WipeDiskFailedReason, msg)
			return actionContinue{
				delay: 10 * time.Second,
			}
		}
		delete(s.scope.HetznerBareMetalHost.Annotations, infrav1.WipeDiskAnnotation)
		record.Eventf(s.scope.HetznerBareMetalHost, "WipeDiskDone", "WipeDisk %v was done. Annotation %q was removed.\n%s",
			sliceOfWwns, infrav1.WipeDiskAnnotation, output)
	}

	// If there is a Linux OS on an other disk, then the reboot after the provisioning
	// will likely fail, because the machine boots into the other operating system.
	// We want detect that early, and not start the provisioning process.
	out := sshClient.DetectLinuxOnAnotherDisk(ctx, s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN())
	if out.Err != nil {
		var exitErr *ssh.ExitError
		if errors.As(out.Err, &exitErr) && exitErr.ExitStatus() > 0 {
			// The script detected Linux on an other disk. This is a permanent error.
			msg := fmt.Sprintf("DetectLinuxOnAnotherDisk failed (permanent error): %s. StdErr: %s (%s)",
				out.StdOut, out.StdErr, out.Err.Error())
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.LinuxOnOtherDiskFoundReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostLinuxOnOtherDiskFoundV1Beta2Reason,
				Message: msg,
			})
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.LinuxOnOtherDiskFoundReason, msg)
			s.scope.HetznerBareMetalHost.SetError(infrav1.PermanentError, msg)
			return actionStop{}
		}

		// Some other error like connection timeout. Retry again later.
		// This often during provisioning.
		msg := fmt.Sprintf("will retry: %s. StdErr: %s (%s)",
			out.StdOut, out.StdErr, out.Err.Error())
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.SSHToRescueSystemFailedReason,
			clusterv1beta1.ConditionSeverityInfo,
			"%s",
			msg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostSSHToRescueSystemFailedV1Beta2Reason,
			Message: msg,
		})
		record.Event(s.scope.HetznerBareMetalHost, infrav1.SSHToRescueSystemFailedReason, msg)
		return actionContinue{
			delay: 10 * time.Second,
		}
	}
	record.Eventf(s.scope.HetznerBareMetalHost, "NoLinuxOnAnotherDisk", "OK, no Linux on another disk:\n%s\n\n%s", out.StdOut, out.StdErr)

	record.Event(s.scope.HetznerBareMetalHost, "InstallImagePreflightCheckSuccessful", "Rescue system reachable, disks look good.")

	autoSetupInput, actionRes := s.createAutoSetupInput(ctx, sshClient)
	if actionRes != nil {
		return actionRes
	}

	autoSetup := buildAutoSetup(s.scope.HetznerBareMetalHost.Spec.Status.InstallImage, autoSetupInput)

	out = sshClient.CreateAutoSetup(ctx, autoSetup)
	if out.Err != nil {
		return actionError{err: fmt.Errorf("failed to create autosetup: %q %q %w", out.StdOut, out.StdErr, out.Err)}
	}

	if out.StdErr != "" {
		return actionError{err: fmt.Errorf("failed to create autosetup: %q %q %w. Content: %s", out.StdOut, out.StdErr, out.Err, autoSetup)}
	}

	// create post install script
	postInstallScript := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.PostInstallScript

	if !strings.HasPrefix(postInstallScript, "#!/bin/bash") {
		postInstallScript = fmt.Sprintf("#!/bin/bash\n%s", postInstallScript)
	}

	cloudInitData, err := s.scope.GetRawBootstrapData(ctx)
	if err != nil {
		return actionError{err: fmt.Errorf("failed to get user data: %w", err)}
	}

	postInstallScript = fmt.Sprintf(`%s

# install cloud-init data

trap 'echo "ERROR: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

mkdir -p /var/lib/cloud/seed/nocloud-net

cat << 'EOF_POST_INSTALL_SCRIPT' > /var/lib/cloud/seed/nocloud-net/meta-data
local-hostname: %s
EOF_POST_INSTALL_SCRIPT

cat << 'EOF_POST_INSTALL_SCRIPT' > /var/lib/cloud/seed/nocloud-net/user-data
%s
EOF_POST_INSTALL_SCRIPT

echo %q
# end of install cloud-init data
`, postInstallScript, s.scope.Hostname(), cloudInitData, PostInstallScriptFinished)

	if err := handleSSHError(sshClient.CreatePostInstallScript(ctx, postInstallScript)); err != nil {
		return actionError{err: fmt.Errorf("failed to create post install script %s: %w", postInstallScript, err)}
	}

	record.Event(s.scope.HetznerBareMetalHost, "InstallingMachineImageStarted",
		s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image.String())

	out = sshClient.UntarTGZ(ctx)
	if out.Err != nil {
		record.Warnf(s.scope.HetznerBareMetalHost, "UntarInstallimageTgzFailed", "err: %s, stderr: %s", out.Err.Error(), out.StdErr)
		return actionError{err: fmt.Errorf("UntarInstallimageTgzFailed: %w", out.Err)}
	}
	record.Event(s.scope.HetznerBareMetalHost, "ExecuteInstallImageStarted",
		s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image.String())

	// Execute install image
	out = sshClient.ExecuteInstallImage(ctx, postInstallScript != "")
	if out.Err != nil {
		record.Warnf(s.scope.HetznerBareMetalHost, "ExecuteInstallImageFailed", out.String())
		return actionError{err: fmt.Errorf("failed to execute installimage: %w", out.Err)}
	}
	s.scope.Info("ExecuteInstallImage started successfully", "out", out.String())
	return actionContinue{delay: 10 * time.Second}
}

func (s *Service) actionImageInstallingFinished(ctx context.Context, sshClient sshclient.Client) actionResult {
	output, err := sshClient.GetResultOfInstallImage(ctx)
	if err != nil {
		return actionError{
			err: fmt.Errorf("GetResultOfInstallImage failed: %w", err),
		}
	}
	if !strings.Contains(output, PostInstallScriptFinished) {
		record.Warn(s.scope.HetznerBareMetalHost, "InstallImageNotSuccessful", output)
		return actionError{err: fmt.Errorf("did not find marker %q in stdout. Installimage was not successful: %s",
			PostInstallScriptFinished, output)}
	}

	record.Event(s.scope.HetznerBareMetalHost, "InstallImageOutput", output)
	s.scope.Info("InstallImageOutput", "output", output)

	// Update name in robot API
	if _, err := s.scope.RobotClient.SetBMServerName(s.scope.HetznerBareMetalHost.Spec.ServerID, s.scope.Hostname()); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			// If the Hetzner API returns this, we just want to retry later:
			// Post "https://robot-ws.your-server.de/server/1234": net/http: TLS handshake timeout
			s.scope.Info("SetBMServerName timed out, will retry later", "error", err)
			return actionContinue{
				delay: 10 * time.Second,
			}
		}
		record.Warn(s.scope.HetznerBareMetalHost, "SetBMServerNameFailed", err.Error())
		s.handleRobotRateLimitExceeded(err, "SetBMServerName")
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	out := sshClient.Reboot(ctx)
	if err := handleSSHError(out); err != nil {
		err = fmt.Errorf("failed to reboot server (after install-image): %w", err)
		record.Warn(s.scope.HetznerBareMetalHost, "RebootFailed", err.Error())
		return actionError{err: err}
	}
	s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
	createSSHRebootEvent(ctx, s.scope.HetznerBareMetalHost, "machine image and cloud-init data got installed")

	s.scope.Info("RebootAfterInstallimageSucceeded", "stdout", out.StdOut, "stderr", out.StdErr)

	// clear potential errors - all done
	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func (s *Service) createAutoSetupInput(ctx context.Context, sshClient sshclient.Client) (autoSetupInput, actionResult) {
	image := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image
	imagePath, needsDownload, errorMessage := image.GetDetails()
	if errorMessage != "" {
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.ImageSpecInvalidReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			errorMessage,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostImageSpecInvalidV1Beta2Reason,
			Message: errorMessage,
		})
		record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostImageSpecInvalidV1Beta2Reason, errorMessage)
		return autoSetupInput{}, s.recordActionFailure(infrav1.ProvisioningError, errorMessage)
	}
	if needsDownload {
		// DownloadImage is a synchronous process. This means the controller waits until the
		// download is finished. Note: We should use StartImageURLCommand(), similar to the handling
		// of ImageURLCommand.
		out := sshClient.DownloadImage(ctx, imagePath, image.URL)
		if err := handleSSHError(out); err != nil {
			err := fmt.Errorf("failed to download image: %s %s %w", out.StdOut, out.StdErr, err)
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.ProvisionSucceededCondition,
				infrav1.ImageDownloadFailedReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				err.Error(),
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostDownloadingImageFailedV1Beta2Reason,
				Message: err.Error(),
			})
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.ImageDownloadFailedReason, err.Error())
			return autoSetupInput{}, actionError{err: err}
		}
	}

	// get the information about storage devices again to have the latest names which are then taken for installimage
	// Device names can change during restart.
	storage, err := obtainHardwareDetailsStorage(ctx, sshClient)
	if err != nil {
		return autoSetupInput{}, actionError{err: fmt.Errorf("failed to obtain hardware details storage: %w", err)}
	}

	// get device names from storage device
	deviceNames := getDeviceNames(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN(), storage)

	// we need at least one storage device
	if len(deviceNames) == 0 {
		msg := "no suitable storage device found"
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.ProvisionSucceededCondition,
			infrav1.NoStorageDeviceFoundReason,
			clusterv1beta1.ConditionSeverityError,
			"%s",
			msg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostNoStorageDeviceFoundV1Beta2Reason,
			Message: msg,
		})
		record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostNoStorageDeviceFoundV1Beta2Reason, msg)
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

func analyzeSSHOutputInstallImage(ctx context.Context, out sshclient.Output, sshClient sshclient.Client, port int) (isTimeout, isConnectionRefused bool, reterr error) {
	// check err
	if out.Err != nil {
		switch {
		case os.IsTimeout(out.Err) || sshclient.IsTimeoutError(out.Err):
			isTimeout = true
			return isTimeout, false, nil
		case sshclient.IsAuthenticationFailedError(out.Err):
			if err := handleAuthenticationFailed(ctx, sshClient, port); err != nil {
				return false, false, fmt.Errorf("original ssh error: %w. err: %w", out.Err, err)
			}
			return false, false, nil
		case sshclient.IsConnectionRefusedError(out.Err):
			return false, verifyConnectionRefused(ctx, sshClient, port), nil
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

func handleAuthenticationFailed(ctx context.Context, sshClient sshclient.Client, port int) error {
	// Check whether we are in the wrong system in the case that rescue and os system might be running on the same port.
	if port == rescuePort {
		if sshClient.GetHostName(ctx).Err == nil {
			// We are in the wrong system, so return false, false, nil
			return nil
		}
	}
	return errWrongSSHKey
}

func verifyConnectionRefused(ctx context.Context, sshClient sshclient.Client, port int) bool {
	// Check whether we are in the wrong system in the case that rescue and os system might be running on the same port.
	if port != rescuePort {
		// Check whether we are in the wrong system
		if sshClient.GetHostName(ctx).Err == nil {
			// We are in the wrong system - this error is not temporary
			return false
		}
	}
	return true
}

// prev: ImageInstalling
// next: Provisioned
func (s *Service) actionEnsureProvisioned(ctx context.Context) (ar actionResult) {
	markProvisionPending(s.scope.HetznerBareMetalHost, infrav1.StateEnsureProvisioned)

	if !s.scope.SSHAfterInstallImageEnabled() {
		// SSH after installimage is disabled for this machine, so we skip the verification phase.
		record.Event(s.scope.HetznerBareMetalHost, "ServerProvisioned", "server successfully provisioned ('ensure-provisioned' was skipped)")
		v1beta1conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:   infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Reason,
		})
		s.scope.HetznerBareMetalHost.ClearError()
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = nil
		return actionComplete{}
	}

	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})

	// Check hostname with sshClient
	wantHostName := s.scope.Hostname()

	out := sshClient.GetHostName(ctx)
	hostname := trimLineBreak(out.StdOut)
	if hostname != wantHostName {
		// give the reboot some time until it takes effect
		if s.hasJustRebooted() {
			s.scope.Info("ensureProvisioned: hasJustRebooted. Retrying...", "hostname", hostname)
			markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
				infrav1.StateEnsureProvisioned, "host has just rebooted")
			return actionContinue{delay: 2 * time.Second}
		}

		isTimeout, isSSHConnectionRefusedError, err := analyzeSSHOutputProvisioned(out)
		if err != nil {
			if errors.Is(err, errUnexpectedHostName) {
				// One possible reason: The machine gets used by a second wl-cluster.
				record.Warnf(s.scope.HetznerBareMetalHost, "UnexpectedHostName",
					"EnsureProvision: wanted %q. %s", wantHostName, err.Error())
			}
			markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
				infrav1.StateEnsureProvisioned, err.Error())
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - actionEnsureProvisioned: %w", err)}
		}

		failed, err := s.handleIncompleteBoot(ctx, false, isTimeout, isSSHConnectionRefusedError)
		if failed {
			msg := "reboot handling failed"
			if err != nil {
				msg = err.Error()
			}
			markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
				infrav1.StateEnsureProvisioned, msg)
			return s.recordActionFailure(infrav1.ProvisioningError, msg)
		}
		if err != nil {
			markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
				infrav1.StateEnsureProvisioned, err.Error())
			return actionError{err: fmt.Errorf(errMsgFailedHandlingIncompleteBoot, err)}
		}
		markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
			infrav1.StateEnsureProvisioned, "will retry")
		return actionContinue{delay: 10 * time.Second}
	}

	// from now on we know that the machine is reachable and
	// is no longer in the rescue system.

	s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = nil

	createEventWithCloudInitOutput := func(ar actionResult) actionResult {
		// Create an Event which contains the cloud-init-output.
		var err error
		switch v := ar.(type) {
		case actionContinue:
			// Do not create and event containing the output, wait until finished.
			return ar
		case actionComplete:
			err = nil
		case actionError:
			err = v.err
		default:
			s.scope.Info("Unhandled type of actionResult",
				"actionResult", ar)
		}
		out := sshClient.GetCloudInitOutput(ctx)
		exitStatus, exitError := out.ExitStatus()
		if exitError != nil {
			err = fmt.Errorf("failed to get cloud init output (ssh connection failed): %w", errors.Join(exitError, err))
			markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
				infrav1.StateEnsureProvisioned, err.Error())
			return actionError{err: err}
		}
		if exitStatus != 0 || out.StdErr != "" {
			err = errors.Join(err, fmt.Errorf("failed to get cloud init output (ssh connection worked): %s",
				out.String()))
		}
		if err != nil {
			record.Warnf(s.scope.HetznerBareMetalHost, "GetCloudInitOutputFailed",
				"GetCloudInitOutput failed to get /var/log/cloud-init-output.log: %s",
				err)
			err = fmt.Errorf("failed to get cloud init output: %w", err)
			markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
				infrav1.StateEnsureProvisioned, err.Error())
			return actionError{err: err}
		}
		record.Eventf(s.scope.HetznerBareMetalHost, "CloudInitOutput", "cloud init output:\n%s",
			out.StdOut)
		return ar
	}

	// Check the status of cloud init
	actResult, msg := s.checkCloudInitStatus(ctx, sshClient)
	if _, complete := actResult.(actionComplete); !complete {
		record.Event(s.scope.HetznerBareMetalHost, "CloudInitStillRunning", msg)
		markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
			infrav1.StateEnsureProvisioned, "cloud-init is still running")
		return createEventWithCloudInitOutput(actResult)
	}

	actResult = s.handleCloudInitNotStarted(ctx)
	if _, complete := actResult.(actionComplete); !complete {
		s.scope.Info("ensureProvisioned: handleCloudInitNotStarted", "actResult", actResult)
		markProvisionPendingWithInfo(s.scope.HetznerBareMetalHost,
			infrav1.StateEnsureProvisioned, "cloud-init has not started yet")
		return createEventWithCloudInitOutput(actResult)
	}

	record.Event(s.scope.HetznerBareMetalHost, "ServerProvisioned", "server successfully provisioned")
	v1beta1conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)
	v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
		Type:   infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Reason,
	})
	s.scope.HetznerBareMetalHost.ClearError()
	return createEventWithCloudInitOutput(actionComplete{})
}

func (s *Service) checkCloudInitStatus(ctx context.Context, sshClient sshclient.Client) (actionResult, string) {
	out := sshClient.CloudInitStatus(ctx)

	status, err := out.ExitStatus()
	if err != nil {
		err = fmt.Errorf("getting CloudInitStatus failed (ssh connection failed): %w", err)
		return actionContinue{delay: 5 * time.Second}, err.Error()
	}

	if status != 0 {
		err = fmt.Errorf("command of CloudInitStatus failed (ssh connection worked): %s",
			out.String())
		return actionError{err: err}, err.Error()
	}

	stdOut := trimLineBreak(out.StdOut)
	switch {
	case strings.Contains(stdOut, "status: running"):
		// Cloud init is still running
		return actionContinue{delay: 5 * time.Second}, "cloud-init still running"

	case strings.Contains(stdOut, "status: disabled"):
		// Reboot needs to be triggered again - did not start yet
		out = sshClient.Reboot(ctx)
		msg := "cloud-init-status was 'disabled'"
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to reboot (%s): %w", msg, err)}, ""
		}
		createSSHRebootEvent(ctx, s.scope.HetznerBareMetalHost, msg)
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTriggered, "ssh reboot just triggered")
		record.Warn(s.scope.HetznerBareMetalHost, "SSHRebootAfterCloudInitStatusDisabled", msg)
		return actionContinue{delay: 5 * time.Second}, "cloud-init was disabled. Triggered a reboot again"

	case strings.Contains(stdOut, "status: done"):
		s.scope.HetznerBareMetalHost.ClearError()
		return actionComplete{}, "cloud-init is done"

	case strings.Contains(stdOut, "status: error"):
		msg := fmt.Sprintf("cloud init returned status error: %s", out.String())
		record.Warn(s.scope.HetznerBareMetalHost, "CloudInitFailed", msg)
		return s.recordActionFailure(infrav1.FatalError, msg), msg

	default:
		err = fmt.Errorf("unknown cloud-init output: %s", out.String())
		return actionError{err: err}, err.Error()
	}
}

func (s *Service) handleCloudInitNotStarted(ctx context.Context) actionResult {
	// Check whether cloud init really was successfully. Sigterm causes problems there.
	oldSSHClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})
	out := oldSSHClient.CheckCloudInitLogsForSigTerm(ctx)
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to CheckCloudInitLogsForSigTerm: %w", err)}
	}

	if trimLineBreak(out.StdOut) != "" {
		// it was not successful. Prepare and reboot again
		out = oldSSHClient.CleanCloudInitLogs(ctx)
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to CleanCloudInitLogs: %w", err)}
		}
		out = oldSSHClient.CleanCloudInitInstances(ctx)
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to CleanCloudInitInstances: %w", err)}
		}
		out = oldSSHClient.Reboot(ctx)
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to reboot (handleCloudInitNotStarted): %w", err)}
		}

		s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())

		createSSHRebootEvent(ctx, s.scope.HetznerBareMetalHost, "machine image and cloud-init data got installed")
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

// previous: EnsureProvisioned
// next: Stays in Provisioned (final state)
//
// actionProvisioned is the steady-state handler for a fully provisioned host.
// It has two responsibilities:
//
//  1. If no reboot annotation is present: clear any leftover reboot state and
//     return actionComplete{}, which keeps the host in the Provisioned state.
//
//  2. If a reboot annotation is present: execute a two-phase reboot cycle.
//     Phase 1:
//     Read the current BootID from the node in the workload cluster, store it,
//     then send the reboot command.
//     Phase 2:
//     On every subsequent reconcile, compare the live BootID of the node from the workload
//     cluster against the stored one. A change means the node completed a full
//     reboot cycle.
func (s *Service) actionProvisioned(ctx context.Context) actionResult {
	rebootDesired := s.scope.HetznerBareMetalHost.HasRebootAnnotation()

	host := s.scope.HetznerBareMetalHost

	// Connect to the workload cluster to read node state.
	wlClient, err := s.scope.WorkloadClusterClientFactory.NewWorkloadClient(ctx)
	if err != nil {
		err = fmt.Errorf("actionProvisioned, failed to get wlClient: %w", err)
		v1beta1conditions.MarkFalse(host,
			infrav1.NodeBootIDRetrievedCondition,
			infrav1.GetWorkloadClusterClientFailedReason,
			clusterv1beta1.ConditionSeverityWarning, "%s",
			err.Error())
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostNodeBootIDRetrievedV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  infrav1.HetznerBareMetalHostGettingWorkloadClusterClientFailedV1Beta2Reason,
			Message: err.Error(),
		})
		return actionError{err: err}
	}

	// Fetch the CAPI Machine that owns the HetznerBareMetalMachine. We need it to
	// look up the Node name in the workload cluster (stored in machine.Status.NodeRef).
	machine, err := util.GetOwnerMachine(ctx, s.scope.Client, s.scope.HetznerBareMetalMachine.ObjectMeta)
	if err != nil {
		err = fmt.Errorf("actionProvisioned, GetOwnerMachine failed: %w",
			err)
		return actionError{err: err}
	}

	if machine == nil {
		err := fmt.Errorf("actionProvisioned, owner Machine for HetznerBareMetalMachine not found")
		return actionError{err: err}
	}

	// NodeRef is expected to be set once the Machine has successfully joined the cluster.
	// If it is still nil at this stage, it likely indicates that the node never registered
	// (e.g. kubelet failed to join, bootstrap issues, etc.) which we treat as a fatal error.
	// The machine would be remediated.
	if machine.Status.NodeRef.Name == "" {
		msg := "machine.Status.NodeRef.Name is empty"

		// Without looking at the node object we can't confirm whether a reboot completed, so that is fatal error.
		// When no reboot is requested the boot ID is non-critical; requeue and wait for kubelet to populate it.
		if rebootDesired {
			s.scope.Error(errors.New(msg), "")
			s.scope.HetznerBareMetalHost.SetError(infrav1.FatalError, msg)
			record.Warn(s.scope.HetznerBareMetalHost, "NodeRefEmpty", msg)
			return actionStop{}
		}

		return actionContinue{}
	}

	nodeName := machine.Status.NodeRef.Name
	node := &corev1.Node{}
	err = wlClient.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		err = fmt.Errorf("failed to get corresponding Node object from the workload cluster: %w", err)
		v1beta1conditions.MarkFalse(
			host,
			infrav1.NodeBootIDRetrievedCondition,
			infrav1.GetNodeInWorkloadClusterFailedReason,
			clusterv1beta1.ConditionSeverityWarning,
			"%s",
			err.Error())
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostNodeBootIDRetrievedV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  infrav1.HetznerBareMetalHostGettingNodeInWorkloadClusterFailedV1Beta2Reason,
			Message: err.Error(),
		})
		return actionError{err: err}
	}

	// The BootID is a random UUID the kubelet sets on startup. Comparing the value
	// before and after a reboot is the way to confirm the node rebooted.
	currentBootID := node.Status.NodeInfo.BootID
	if currentBootID == "" {
		msg := "node.Status.NodeInfo.BootID is empty"
		v1beta1conditions.MarkFalse(host,
			infrav1.NodeBootIDRetrievedCondition,
			infrav1.BootIDEmptyReason,
			clusterv1beta1.ConditionSeverityWarning,
			"%s",
			msg)
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostNodeBootIDRetrievedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostBootIDEmptyV1Beta2Reason,
			Message: msg,
		})

		s.scope.Error(errors.New(msg), "")

		// Without a boot ID we can't confirm a reboot completed, so that is fatal error.
		// When no reboot is requested the boot ID is non-critical; requeue and wait for kubelet to populate it.
		if rebootDesired {
			s.scope.HetznerBareMetalHost.SetError(infrav1.FatalError, msg)
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostBootIDEmptyV1Beta2Reason, msg)
			return actionStop{}
		}

		return actionContinue{}
	}

	v1beta1conditions.MarkTrue(host, infrav1.NodeBootIDRetrievedCondition)
	v1beta2conditions.Set(host, metav1.Condition{
		Type:   infrav1.HetznerBareMetalHostNodeBootIDRetrievedV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HetznerBareMetalHostNodeBootIDRetrievedV1Beta2Reason,
	})

	if !rebootDesired {
		// No reboot annotation, ensure all reboot-related state is cleared.
		host.Spec.Status.Rebooted = false
		host.Spec.Status.RebootTriggeredAt = nil

		// Populate NodeBootID the first time the host enters Provisioned state.
		if host.Spec.Status.NodeBootID == "" {
			host.Spec.Status.NodeBootID = currentBootID
		}

		return actionFinished{}
	}

	// --- Reboot via annotation ---

	// The hard part is detecting when the node is back up after the reboot.
	// We do this by watching node.Status.NodeInfo.BootID in the workload cluster:
	// the kubelet sets a fresh random BootID on every boot, so a changed value
	// is a signal that a full reboot cycle completed.

	// Enforce the overall reboot timeout. If the BootID has not changed within
	// 5 minutes of the annotation being set, something went wrong and we return an error.
	if host.Spec.Status.RebootTriggeredAt != nil {
		rebootDuration := time.Since(host.Spec.Status.RebootTriggeredAt.Time)
		if rebootDuration > 5*time.Minute {
			msg := fmt.Sprintf("Rebooting timed out after: %s", rebootDuration.Round(time.Second))
			s.scope.Info(msg)
			record.Warn(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostRebootSucceededTimeoutReachedOutV1Beta2Reason, msg)
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.RebootSucceededCondition,
				"TimedOut",
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostRebootSucceededTimeoutReachedOutV1Beta2Reason,
				Message: msg,
			})
			return s.recordActionFailure(infrav1.FatalError, msg)
		}
	}

	if !host.Spec.Status.Rebooted {
		// --- Phase 1: trigger the reboot ---
		// This branch runs exactly once per reboot annotation. We store the current
		// BootID so Phase 2 can detect when it changes, then send the reboot command.

		msg := fmt.Sprintf("Rebooting because annotation was set. Old BootID: %s", currentBootID)

		if s.scope.SSHAfterInstallImageEnabled() {
			// SSH-based reboot: issue a reboot command directly over SSH.
			creds := sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, host.Spec.Status.SSHSpec.SecretRef)

			in := sshclient.Input{
				PrivateKey: creds.PrivateKey,
				Port:       host.Spec.Status.SSHSpec.PortAfterInstallImage,
				IP:         host.Spec.Status.GetIPAddress(),
			}

			sshClient := s.scope.SSHClientFactory.NewClient(in)

			out := sshClient.Reboot(ctx)
			if err := handleSSHError(out); err != nil {
				v1beta1conditions.MarkFalse(host, infrav1.RebootSucceededCondition,
					"RebootViaSSHFailed",
					clusterv1beta1.ConditionSeverityWarning, "%s",
					err.Error())
				v1beta2conditions.Set(host, metav1.Condition{
					Type:    infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Condition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav1.HetznerBareMetalHostRebootingViaSSHFailedV1Beta2Reason,
					Message: err.Error(),
				})
				return actionError{err: err}
			}

			createSSHRebootEvent(ctx, host, msg)
		} else {
			// Hardware reboot: trigger via the Hetzner Robot API.
			rebootType := infrav1.RebootTypeHardware
			if _, err := s.scope.RobotClient.RebootBMServer(host.Spec.ServerID, rebootType); err != nil {
				// If Robot API returned "unauthorized" error - mark condition RobotCredentialsAvailable as false
				// with reason RobotCredentialsInvalidReason and stop reconciling.
				if models.IsError(err, models.ErrorCodeUnauthorized) {
					msg := "Robot API returned unauthorized; verify the credentials in the referenced secret are correct"
					v1beta1conditions.MarkFalse(
						s.scope.HetznerBareMetalHost,
						infrav1.RobotCredentialsAvailableCondition,
						infrav1.RobotCredentialsInvalidReason,
						clusterv1beta1.ConditionSeverityError,
						"%s",
						msg,
					)
					v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
						Type:    infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
						Status:  metav1.ConditionFalse,
						Reason:  infrav1.HetznerBareMetalHostRobotCredentialsInvalidV1Beta2Reason,
						Message: msg,
					})
					record.Warnf(s.scope.HetznerBareMetalHost, infrav1.RobotCredentialsInvalidReason, msg)

					return actionStop{}
				}

				s.handleRobotRateLimitExceeded(err, rebootServerStr)

				err = fmt.Errorf("actionProvisioned (Reboot via Annotation), reboot (%s) failed: %w", rebootType, err)

				v1beta1conditions.MarkFalse(host, infrav1.RebootSucceededCondition,
					"RebootBMServerViaAPIFailed",
					clusterv1beta1.ConditionSeverityWarning, "%s",
					err.Error())
				v1beta2conditions.Set(host, metav1.Condition{
					Type:    infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Condition,
					Status:  metav1.ConditionFalse,
					Reason:  infrav1.HetznerBareMetalHostRebootingBMServerViaAPIFailedV1Beta2Reason,
					Message: err.Error(),
				})
				return actionError{err: err}
			}

			createHardwareRebootEvent(ctx, host, msg)

			v1beta1conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RobotCredentialsAvailableCondition)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:   infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason,
			})
		}

		// Persist the pre-reboot BootID. Phase 2 compares the live BootID against this
		// value on every reconcile; a difference means the node completed a reboot.
		host.Spec.Status.NodeBootID = currentBootID
		host.Spec.Status.RebootTriggeredAt = ptr.To(metav1.Now())
		host.Spec.Status.Rebooted = true

		v1beta1conditions.MarkFalse(host, infrav1.RebootSucceededCondition,
			"WaitingForNodeToBeRebooted",
			clusterv1beta1.ConditionSeverityInfo, "%s",
			msg)
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostRebootingV1Beta2Reason,
			Message: msg,
		})
		return actionContinue{delay: 30 * time.Second}
	}

	// --- Phase 2: verify the reboot ---
	// The reboot command was already sent in a previous reconcile.
	// Poll until we confirm the node came back up.

	// Compare the current BootID against the one we stored in Phase 1.
	// A change signals that the node completed a full reboot cycle.
	if host.Spec.Status.NodeBootID != currentBootID {
		// Reboot has been successful
		s.scope.Info(fmt.Sprintf("BootID changed: %q -> %q", host.Spec.Status.NodeBootID, currentBootID))
		host.Spec.Status.RebootTriggeredAt = nil
		host.Spec.Status.Rebooted = false

		v1beta1conditions.MarkTrue(host, infrav1.RebootSucceededCondition)
		v1beta2conditions.Set(host, metav1.Condition{
			Type:   infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Reason,
		})

		host.ClearRebootAnnotations()
		host.ClearError()

		return actionFinished{}
	}

	// BootID has not changed yet. The node is either still rebooting or the reboot
	// command hasn't taken effect yet.
	v1beta1conditions.MarkFalse(host, infrav1.RebootSucceededCondition,
		"WaitingForNodeToBeRebooted",
		clusterv1beta1.ConditionSeverityInfo,
		"Waiting for the node to be rebooted",
	)
	v1beta2conditions.Set(host, metav1.Condition{
		Type:    infrav1.HetznerBareMetalHostRebootSucceededV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HetznerBareMetalHostRebootingV1Beta2Reason,
		Message: "Waiting for the node to be rebooted",
	})

	return actionContinue{delay: 10 * time.Second}
}

// next: None
func (s *Service) actionDeprovisioning(ctx context.Context) actionResult {
	// remove the reboot annotation if present.
	s.scope.HetznerBareMetalHost.ClearRebootAnnotations()

	// remove the RebootSucceeded condition if present.
	v1beta1conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.RebootSucceededCondition)

	// Update server name via RobotAPI, strip "bm-" from the desired hostname.
	// Example: If the hostname is "bm-abc-1-2356799" it should be renamed to "abc-1-2356799".
	if _, err := s.scope.RobotClient.SetBMServerName(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		strings.TrimPrefix(s.scope.Hostname(), infrav1.BareMetalHostNamePrefix),
	); err != nil {
		if models.IsError(err, models.ErrorCodeUnauthorized) {
			// If Robot API returned "unauthorized" error while trying to set baremetal server name, then
			// mark condition RobotCredentialsAvailable as false with reason RobotCredentialsInvalid
			// and stop reconciling.
			msg := "Robot API returned unauthorized; verify the credentials in the referenced secret are correct"
			v1beta1conditions.MarkFalse(
				s.scope.HetznerBareMetalHost,
				infrav1.RobotCredentialsAvailableCondition,
				infrav1.RobotCredentialsInvalidReason,
				clusterv1beta1.ConditionSeverityError,
				"%s",
				msg,
			)
			v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
				Type:    infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HetznerBareMetalHostRobotCredentialsInvalidV1Beta2Reason,
				Message: msg,
			})
			record.Warnf(s.scope.HetznerBareMetalHost, infrav1.RobotCredentialsInvalidReason, msg)

			return actionStop{}
		}

		s.handleRobotRateLimitExceeded(err, "SetBMServerName")
		if models.IsError(err, models.ErrorCodeServerNotFound) {
			msg := "server not found in Robot API during deprovisioning, assuming already removed"
			s.scope.Info(msg)
			// Clear previous errors/conditions so deletion can finish.
			s.scope.HetznerBareMetalHost.ClearError()
			v1beta1conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)
			v1beta2conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition)
			return actionComplete{}
		}
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	v1beta1conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RobotCredentialsAvailableCondition)
	v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
		Type:   infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason,
	})

	if s.scope.SSHAfterInstallImageEnabled() {
		// If it has been provisioned completely, stop all running pods
		if s.scope.OSSSHSecret != nil {
			sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
				PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
				Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
				IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
			})
			out := sshClient.ResetKubeadm(ctx)
			s.scope.V(1).Info("Output of ResetKubeadm", "stdout", out.StdOut, "stderr", out.StdErr, "err", out.Err)
			if out.Err != nil {
				record.Warnf(s.scope.HetznerBareMetalHost, "FailedResetKubeAdm", "failed to reset kubeadm: %s", out.Err.Error())
			} else {
				record.Event(s.scope.HetznerBareMetalHost, "SuccessfulResetKubeAdm", "Reset was successful.")
			}
		} else {
			s.scope.Info("OS SSH Secret is empty - cannot reset kubeadm")
		}
	}

	// Only keep permanent errors on the host object after deprovisioning.
	// Permanent errors are those ones that do not get solved with de- or re-provisioning.
	if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType != infrav1.PermanentError {
		s.scope.HetznerBareMetalHost.ClearError()
	}

	// Always clear the ProvisionSucceeded condition during deprovisioning to avoid a misleading
	// StillProvisioning condition with an empty state when a permanent error occur.
	// The permanent error details remain in status.ErrorMessage.
	v1beta1conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.ProvisionSucceededCondition)
	v1beta2conditions.Delete(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition)
	return actionComplete{} // next: None
}

func (s *Service) actionDeleting(_ context.Context) actionResult {
	controllerutil.RemoveFinalizer(s.scope.HetznerBareMetalHost, infrav1.HetznerBareMetalHostFinalizer)
	controllerutil.RemoveFinalizer(s.scope.HetznerBareMetalHost, infrav1.DeprecatedBareMetalHostFinalizer)
	return deleteComplete{}
}

func (s *Service) handleRobotRateLimitExceeded(err error, functionName string) {
	if models.IsError(err, models.ErrorCodeRateLimitExceeded) || strings.Contains(err.Error(), "server responded with status code 403") {
		msg := fmt.Sprintf("exceeded robot rate limit with calling function %q: %s", functionName, err.Error())
		v1beta1conditions.MarkFalse(
			s.scope.HetznerBareMetalHost,
			infrav1.HetznerAPIReachableCondition,
			infrav1.RateLimitExceededReason,
			clusterv1beta1.ConditionSeverityWarning,
			"%s",
			msg,
		)
		v1beta2conditions.Set(s.scope.HetznerBareMetalHost, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRobotRateLimitExceededV1Beta2Condition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav1.HetznerBareMetalHostRobotRateLimitExceededV1Beta2Reason,
			Message: msg,
		})
		record.Warnf(s.scope.HetznerBareMetalHost, "RateLimitExceeded", msg)
	}
}

// hasJustRebooted returns true if a reboot was done during the last seconds.
// The method gets used to let the controller wait until the reboot was actually done.
// Imagine the controller triggers a reboot, and reconciles immediately. This would
// mean the controller would do the same reboot immediately again.
func (s *Service) hasJustRebooted() bool {
	// Safe guard: RebootTriggeredAt should not be nil, when hasJustRebooted() gets called. If
	// RebootTriggeredAt is nil, we cannot know when the reboot happened, so we treat it as not just
	// rebooted. Without this guard, hasTimedOut(nil, ...) returns false, making this function
	// return true indefinitely.
	if s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt == nil {
		s.scope.Info("hasJustRebooted: s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt is nil. That is not expected")
		return false
	}
	return (s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeSSHRebootTriggered ||
		s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeSoftwareRebootTriggered ||
		s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeHardwareRebootTriggered) &&
		!hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.RebootTriggeredAt, rebootWaitTime)
}

func markProvisionPendingWithInfo(host *infrav1.HetznerBareMetalHost, state infrav1.ProvisioningState, info string) {
	msg := fmt.Sprintf("host (%s) is still provisioning - state %q", host.Name, state)
	if info != "" {
		msg = fmt.Sprintf("%s: %s", msg, info)
	}
	v1beta1conditions.MarkFalse(
		host,
		infrav1.ProvisionSucceededCondition,
		infrav1.StillProvisioningReason,
		clusterv1beta1.ConditionSeverityInfo,
		"%s", msg,
	)
	v1beta2conditions.Set(host, metav1.Condition{
		Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
		Status:  metav1.ConditionFalse,
		Reason:  infrav1.HetznerBareMetalHostProvisioningV1Beta2Reason,
		Message: msg,
	})
}

func markProvisionPending(host *infrav1.HetznerBareMetalHost, state infrav1.ProvisioningState) {
	markProvisionPendingWithInfo(host, state, "")
}

func createSSHRebootEvent(ctx context.Context, host *infrav1.HetznerBareMetalHost, msg string) {
	createRebootEvent(ctx, host, infrav1.RebootTypeSSH, msg)
}

func createHardwareRebootEvent(ctx context.Context, host *infrav1.HetznerBareMetalHost, msg string) {
	createRebootEvent(ctx, host, infrav1.RebootTypeHardware, msg)
}

func createRebootEvent(ctx context.Context, host *infrav1.HetznerBareMetalHost, rebootType infrav1.RebootType, msg string) string {
	verboseRebootType := infrav1.VerboseRebootType(rebootType)
	reason := fmt.Sprintf("RebootBMServerVia%sProvisioningState%s",
		verboseRebootType,
		strcase.UpperCamelCase(string(host.Spec.Status.ProvisioningState)))
	msg = fmt.Sprintf("Phase %s, reboot via %s: %s", host.Spec.Status.ProvisioningState, verboseRebootType, msg)
	record.Eventf(host, reason, msg)
	logger := ctrl.LoggerFrom(ctx)
	logger.Info(msg, "reason", reason, "host", host.Name)
	return msg
}
