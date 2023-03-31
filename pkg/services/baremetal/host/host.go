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
	"strconv"
	"strings"
	"time"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// hoursBeforeDeletion      time.Duration = 36
	// rateLimitTimeOut         time.Duration = 660
	// rateLimitTimeOutDeletion time.Duration = 120.

	sshResetTimeout      time.Duration = 5 * time.Minute
	softwareResetTimeout time.Duration = 5 * time.Minute
	hardwareResetTimeout time.Duration = 60 * time.Minute
	rescue               string        = "rescue"
	rescuePort           int           = 22
)

// Service defines struct with machine scope to reconcile HetznerBareMetalHosts.
type Service struct {
	scope *scope.BareMetalHostScope
}

// NewService outs a new service with machine scope.
func NewService(scope *scope.BareMetalHostScope) *Service {
	return &Service{
		scope: scope,
	}
}

// Reconcile implements reconcilement of HetznerBareMetalHosts.
func (s *Service) Reconcile(ctx context.Context) (result ctrl.Result, err error) {
	s.scope.Info("Reconciling baremetal host", "name", s.scope.HetznerBareMetalHost.Name)

	initialState := s.scope.HetznerBareMetalHost.Spec.Status.ProvisioningState

	oldHost := *s.scope.HetznerBareMetalHost

	hostStateMachine := newHostStateMachine(s.scope.HetznerBareMetalHost, s, s.scope.Logger)

	// reconcile state
	actResult := hostStateMachine.ReconcileState(ctx)
	result, err = actResult.Result()
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("action %q failed: %w", initialState, err)
	}

	// save host if it changed during reconciliation
	if !reflect.DeepEqual(oldHost, *s.scope.HetznerBareMetalHost) {
		return SaveHostAndReturn(ctx, s.scope.Client, s.scope.HetznerBareMetalHost)
	}

	return result, nil
}

func (s *Service) recordActionFailure(errorType infrav1.ErrorType, errorMessage string) actionFailed {
	s.scope.HetznerBareMetalHost.SetError(errorType, errorMessage)
	s.scope.Error(errors.New("action failure"), errorMessage, "errorType", errorType)
	return actionFailed{ErrorType: errorType, errorCount: s.scope.HetznerBareMetalHost.Spec.Status.ErrorCount}
}

// SaveHostAndReturn saves host object, updates LastUpdated in host status and returns the reconcile Result.
func SaveHostAndReturn(ctx context.Context, client client.Client, host *infrav1.HetznerBareMetalHost) (res reconcile.Result, err error) {
	t := metav1.Now()
	host.Spec.Status.LastUpdated = &t

	if err := client.Update(ctx, host); err != nil {
		if apierrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return res, fmt.Errorf("failed to update host object: %w", err)
	}
	return res, nil
}

func (s *Service) getSSHKeysAndUpdateStatus() (osSSHSecret *corev1.Secret, rescueSSHSecret *corev1.Secret, err error) {
	// Set ssh status if none has been set so far
	osSSHSecret = s.scope.OSSSHSecret
	rescueSSHSecret = s.scope.RescueSSHSecret
	// If os ssh secret is set and the status not yet, then update it
	if s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.CurrentOS == nil && osSSHSecret != nil {
		if err := s.scope.HetznerBareMetalHost.UpdateOSSSHStatus(*osSSHSecret); err != nil {
			return nil, nil, fmt.Errorf("failed to update OS SSH secret status: %w", err)
		}
	}
	if s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.CurrentRescue == nil && rescueSSHSecret != nil {
		if err := s.scope.HetznerBareMetalHost.UpdateRescueSSHStatus(*rescueSSHSecret); err != nil {
			return nil, nil, fmt.Errorf("failed to update rescue SSH secret status: %w", err)
		}
	}
	return osSSHSecret, rescueSSHSecret, nil
}

func (s *Service) validateSSHKey(sshSecret *corev1.Secret, secretType string) actionResult {
	var secretRef infrav1.SSHSecretRef
	switch secretType {
	case rescue:
		secretRef = s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef
	case "os":
		secretRef = s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef
	}
	creds := sshclient.Credentials{
		Name:       string(sshSecret.Data[secretRef.Key.Name]),
		PublicKey:  string(sshSecret.Data[secretRef.Key.PublicKey]),
		PrivateKey: string(sshSecret.Data[secretRef.Key.PrivateKey]),
	}

	// Validate token
	if err := creds.Validate(); err != nil {
		return s.recordActionFailure(infrav1.PreparationError, infrav1.ErrorMessageMissingOrInvalidSecretData)
	}

	return actionComplete{}
}

func (s *Service) actionPreparing() actionResult {
	server, err := s.scope.RobotClient.GetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		if models.IsError(err, models.ErrorCodeServerNotFound) {
			return s.recordActionFailure(
				infrav1.RegistrationError,
				fmt.Sprintf("bare metal host with id %v not found", s.scope.HetznerBareMetalHost.Spec.ServerID),
			)
		}
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function GetBMServer",
			)
			return actionContinue{}
		}
		return actionError{err: fmt.Errorf("failed to get bare metal server: %w", err)}
	}

	s.scope.HetznerBareMetalHost.Spec.Status.IPv4 = server.ServerIP
	s.scope.HetznerBareMetalHost.Spec.Status.IPv6 = server.ServerIPv6Net + "1"

	sshKey, actResult := s.ensureSSHKey(s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef, s.scope.RescueSSHSecret)
	if _, complete := actResult.(actionComplete); !complete {
		return actResult
	}

	s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey = &sshKey

	// Populate reboot methods in status
	if len(s.scope.HetznerBareMetalHost.Spec.Status.RebootTypes) == 0 {
		reboot, err := s.scope.RobotClient.GetReboot(s.scope.HetznerBareMetalHost.Spec.ServerID)
		if err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function GetReboot",
				)
				return actionContinue{}
			}
			return actionError{err: fmt.Errorf("failed to get reboot: %w", err)}
		}
		var rebootTypes []infrav1.RebootType
		b, err := json.Marshal(reboot.Type)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to marshal: %w", err)}
		}
		if err := json.Unmarshal(b, &rebootTypes); err != nil {
			return actionError{err: fmt.Errorf("failed to unmarshal: %w", err)}
		}
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTypes = rebootTypes
	}

	// Start rescue mode and reboot server if necessary
	if !server.Rescue {
		return s.recordActionFailure(infrav1.RegistrationError, "rescue system not available for server")
	}

	// Delete old rescue activations if exist, as the ssh key might have changed in between
	if _, err := s.scope.RobotClient.DeleteBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID); err != nil {
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function DeleteBootRescue",
			)
			return actionContinue{}
		}
		return actionError{err: fmt.Errorf("failed to delete boot rescue: %w", err)}
	}

	if _, err := s.scope.RobotClient.SetBootRescue(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey.Fingerprint,
	); err != nil {
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function SetBootRescue",
			)
		}
		return actionError{err: fmt.Errorf("failed to set boot rescue: %w", err)}
	}

	var rebootType infrav1.RebootType
	switch {
	case s.scope.HetznerBareMetalHost.HasSoftwareReboot():
		rebootType = infrav1.RebootTypeSoftware
	case s.scope.HetznerBareMetalHost.HasHardwareReboot():
		rebootType = infrav1.RebootTypeHardware
	default:
		return actionError{err: errors.New("no software or hardware reboot available for host")}
	}

	if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function RebootBMServer",
			)
			return actionContinue{}
		}
		return actionError{err: fmt.Errorf("failed to reboot bare metal server: %w", err)}
	}

	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func (s *Service) ensureSSHKey(sshSecretRef infrav1.SSHSecretRef, sshSecret *corev1.Secret) (infrav1.SSHKey, actionResult) {
	if sshSecret == nil {
		return infrav1.SSHKey{}, actionError{err: fmt.Errorf("ssh secret is nil")}
	}
	hetznerSSHKeys, err := s.scope.RobotClient.ListSSHKeys()
	if err != nil {
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function ListSSHKeys",
			)
			return infrav1.SSHKey{}, actionContinue{}
		}
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
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function SetSSHKey",
				)
				return infrav1.SSHKey{}, actionContinue{}
			}
			if models.IsError(err, models.ErrorCodeKeyAlreadyExists) {
				record.Warnf(s.scope.HetznerBareMetalHost, "SSHKeyAlreadyExists",
					"failed to upload ssh key %s - it already exists under a different name", string(sshSecret.Data[sshSecretRef.Key.Name]))
				return infrav1.SSHKey{}, s.recordActionFailure(infrav1.FatalError, "cannot upload ssh key - exists already under a different name")
			}
			return infrav1.SSHKey{}, actionError{err: fmt.Errorf("failed to set ssh key: %w", err)}
		}
		sshKey.Name = hetznerSSHKey.Name
		sshKey.Fingerprint = hetznerSSHKey.Fingerprint
	}
	return sshKey, actionComplete{}
}

func (s *Service) handleIncompleteBootError(isRebootIntoRescue bool, isTimeout bool, isConnectionRefused bool) error {
	if isConnectionRefused {
		if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
			if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, time.Minute) {
				return errors.New("connection refused error of ssh. Might be due to wrong port")
			}
		} else {
			s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeConnectionError, "ssh gave connection error")
		}
		return nil
	} else if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
		// Remove error if it is of type connection error
		s.scope.HetznerBareMetalHost.ClearError()
	}
	// Check whether there has been an error message already, meaning that the reboot did not finish in time
	var emptyErrorType infrav1.ErrorType
	switch s.scope.HetznerBareMetalHost.Spec.Status.ErrorType {
	case emptyErrorType:
		if isTimeout {
			// Reset was too slow - set error message
			s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTooSlow, "ssh timeout error - server has not restarted yet")
			return nil
		}

		// The (ssh) reboot did not start This triggers trigger an API reboot.
		return s.handleErrorTypeSSHRebootNotStarted(!isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeSSHRebootTooSlow:
		return s.handleErrorTypeSSHRebootTooSlow(isTimeout)

	case infrav1.ErrorTypeSoftwareRebootTooSlow:
		return s.handleErrorTypeSoftwareRebootTooSlow(isTimeout)

	case infrav1.ErrorTypeHardwareRebootTooSlow:
		return s.handleErrorTypeHardwareRebootTooSlow(isTimeout)

	case infrav1.ErrorTypeHardwareRebootFailed:
		return s.handleErrorTypeHardwareRebootFailed()

	case infrav1.ErrorTypeSSHRebootNotStarted:
		return s.handleErrorTypeSSHRebootNotStarted(!isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeSoftwareRebootNotStarted:
		return s.handleErrorTypeSoftwareRebootNotStarted(!isTimeout, isRebootIntoRescue)

	case infrav1.ErrorTypeHardwareRebootNotStarted:
		return s.handleErrorTypeHardwareRebootNotStarted(!isTimeout, isRebootIntoRescue)
	}

	return nil
}

func (s *Service) handleErrorTypeSSHRebootTooSlow(isTimeout bool) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, sshResetTimeout) {
		// Perform software or hardware reboot
		var rebootType infrav1.RebootType
		var errorType infrav1.ErrorType
		switch {
		case s.scope.HetznerBareMetalHost.HasSoftwareReboot():
			rebootType = infrav1.RebootTypeSoftware
			errorType = infrav1.ErrorTypeSoftwareRebootNotStarted
		case s.scope.HetznerBareMetalHost.HasHardwareReboot():
			rebootType = infrav1.RebootTypeHardware
			errorType = infrav1.ErrorTypeHardwareRebootNotStarted
		default:
			return errors.New("no software or hardware reboot available for host")
		}

		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}
		// Set error message that software reboot is too slow as we perform this reboot now
		s.scope.HetznerBareMetalHost.SetError(errorType, "ssh reboot timed out")
	}
	if !isTimeout {
		// If it is not a timeout error, then it means we are in the wrong system - not that the reboot is too slow
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootNotStarted, "ssh reboot not timed out - wrong boot")
	}
	return nil
}

func (s *Service) handleErrorTypeSoftwareRebootTooSlow(isTimeout bool) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, softwareResetTimeout) {
		// Perform hardware reboot
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}
		// Set error message that hardware reboot is too slow as we perform this reboot now
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootTooSlow, "software reboot timed out")
	}
	if !isTimeout {
		// If it is not a timeout error, then it means we are in the wrong system - not that the reboot is too slow
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSoftwareRebootNotStarted, "software reboot not timed out - wrong boot")
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareRebootTooSlow(isTimeout bool) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootFailed, "hardware reboot timed out")
		// Perform hardware reboot
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}
	}
	if !isTimeout {
		// If it is not a timeout error, then it means we are in the wrong system - not that the reboot is too slow
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootNotStarted, "hardware reboot not timed out - wrong boot")
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareRebootFailed() error {
	// If a hardware reboot fails we have no option but to trigger a new one if the timeout has been reached.
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootFailed, "hardware reboot failed")
	}
	return nil
}

func (s *Service) handleErrorTypeSSHRebootNotStarted(isInWrongBoot bool, wantsRescue bool) error {
	// Check whether ssh reboot has not been started again and escalate if not.
	// Otherwise set a new error as the ssh reboot has just been slow.
	if isInWrongBoot {
		if wantsRescue {
			if err := s.ensureRescueMode(); err != nil {
				return fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}
		var rebootType infrav1.RebootType
		var errorType infrav1.ErrorType
		switch {
		case s.scope.HetznerBareMetalHost.HasSoftwareReboot():
			rebootType = infrav1.RebootTypeSoftware
			errorType = infrav1.ErrorTypeSoftwareRebootNotStarted
		case s.scope.HetznerBareMetalHost.HasHardwareReboot():
			rebootType = infrav1.RebootTypeHardware
			errorType = infrav1.ErrorTypeHardwareRebootNotStarted
		default:
			return errors.New("no software or hardware reboot available for host")
		}

		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}

		// set an error that software reboot failed to manage further states. If the software reboot started successfully
		// Then we will complete this or go to ErrorStateSoftwareResetTooSlow as expected.
		s.scope.HetznerBareMetalHost.SetError(errorType, "software/hardware reboot triggered after ssh reboot did not start")
	} else {
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootTooSlow, "ssh reboot too slow")
	}
	return nil
}

func (s *Service) handleErrorTypeSoftwareRebootNotStarted(isInWrongBoot bool, wantsRescue bool) error {
	// Check whether software reboot has not been started again and escalate if not.
	// Otherwise set a new error as the software reboot has been slow anyway.
	if isInWrongBoot {
		if wantsRescue {
			if err := s.ensureRescueMode(); err != nil {
				return fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}

		// set an error that hardware reboot not started to manage further states. If the hardware reboot started successfully
		// Then we will complete this or go to ErrorStateHardwareResetTooSlow as expected.
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootNotStarted, "hardware reboot triggered after software reboot did not start")
	} else {
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSoftwareRebootTooSlow, "software reboot too slow")
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareRebootNotStarted(isInWrongBoot bool, wantsRescue bool) error {
	// Check whether software reboot has not been started again and escalate if not.
	// Otherwise set a new error as the software reboot has been slow anyway.
	if isInWrongBoot {
		if wantsRescue {
			if err := s.ensureRescueMode(); err != nil {
				return fmt.Errorf("failed to ensure rescue mode: %w", err)
			}
		}
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function RebootBMServer",
				)
				return nil
			}
			return fmt.Errorf("failed to reboot bare metal server: %w", err)
		}
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootNotStarted, "hardware reboot not started")
	} else {
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeHardwareRebootTooSlow, "hardware reboot too slow")
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
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function GetBootRescue",
			)
			return nil
		}
		return fmt.Errorf("failed to get bare metal server: %w", err)
	}
	if !rescue.Active {
		// Rescue system is still not active - activate again
		s.scope.Info("Rescue system not active - activate again")
		if _, err := s.scope.RobotClient.SetBootRescue(
			s.scope.HetznerBareMetalHost.Spec.ServerID,
			s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey.Fingerprint,
		); err != nil {
			if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
				conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
				record.Event(s.scope.HetznerBareMetalHost,
					"RateLimitExceeded",
					"exceeded rate limit with calling robot function SetBootRescue",
				)
				return nil
			}
			return fmt.Errorf("failed to set boot rescue: %w", err)
		}
	}
	return nil
}

func (s *Service) actionRegistering() actionResult {
	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	// Check hostname with sshClient
	out := sshClient.GetHostName()
	if trimLineBreak(out.StdOut) != rescue {
		isTimeout, isConnectionFailed, err := s.handleIncompleteBootRegistering(out)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - registering: %w", err)}
		}

		if err := s.handleIncompleteBootError(true, isTimeout, isConnectionFailed); err != nil {
			return actionError{err: fmt.Errorf("failed to handle incomplete boot: %w", err)}
		}
		return actionContinue{delay: 10 * time.Second}
	}

	if s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails == nil {
		var hardwareDetails infrav1.HardwareDetails

		mebiBytes, err := s.obtainHardwareDetailsRAM(sshClient)
		if err != nil {
			return actionError{err: err}
		}
		hardwareDetails.RAMGB = mebiBytes / 1000

		nics, err := s.obtainHardwareDetailsNics(sshClient)
		if err != nil {
			return actionError{err: err}
		}
		hardwareDetails.NIC = nics

		storage, err := s.obtainHardwareDetailsStorage(sshClient)
		if err != nil {
			return actionError{err: err}
		}
		hardwareDetails.Storage = storage

		cpu, err := s.obtainHardwareDetailsCPU(sshClient)
		if err != nil {
			return actionError{err: err}
		}
		hardwareDetails.CPU = cpu

		s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails = &hardwareDetails
	}
	if s.scope.HetznerBareMetalHost.Spec.RootDeviceHints == nil ||
		!s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.IsValid() {
		return s.recordActionFailure(infrav1.RegistrationError, infrav1.ErrorMessageMissingRootDeviceHints)
	}

	for _, wwn := range s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN() {
		foundWWN := false
		for _, st := range s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails.Storage {
			if wwn == st.WWN {
				foundWWN = true
				continue
			}
		}
		if !foundWWN {
			return s.recordActionFailure(infrav1.RegistrationError, fmt.Sprintf("no storage device found with root device hint %s", wwn))
		}
	}

	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func (s *Service) handleIncompleteBootRegistering(out sshclient.Output) (isTimeout bool, isConnectionRefused bool, reterr error) {
	if out.Err != nil {
		switch {
		case os.IsTimeout(out.Err) || sshclient.IsTimeoutError(out.Err):
			isTimeout = true
		case sshclient.IsAuthenticationFailedError(out.Err):
			// Check if the reboot did not trigger.
			rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
			if err != nil {
				if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
					conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
					record.Event(s.scope.HetznerBareMetalHost,
						"RateLimitExceeded",
						"exceeded rate limit with calling robot function GetBootRescue",
					)
					return
				}
				reterr = fmt.Errorf("failed to get bare metal server: %w", err)
				return
			}
			if rescue.Active {
				// Reboot did not trigger
				return
			}
			reterr = fmt.Errorf("wrong ssh key: %w", out.Err)
		case sshclient.IsConnectionRefusedError(out.Err):
			// Check if the reboot did not trigger.
			rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
			if err != nil {
				if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
					conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
					record.Event(s.scope.HetznerBareMetalHost,
						"RateLimitExceeded",
						"exceeded rate limit with calling robot function GetBootRescue",
					)
					return
				}
				reterr = fmt.Errorf("failed to get bare metal server: %w", err)
				return
			}
			if rescue.Active {
				// Reboot did not trigger
				return
			}
			isConnectionRefused = true

		default:
			reterr = fmt.Errorf("unhandled ssh error while getting hostname: %w", out.Err)
		}
		return
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		reterr = fmt.Errorf("failed to get host name via ssh. StdErr: %s", out.StdErr)
		return
	}

	// check stdout
	if trimLineBreak(out.StdOut) == "" {
		// Hostname should not be empty. This is unexpected.
		reterr = errors.New("error empty hostname")
	}
	return isTimeout, isConnectionRefused, reterr
}

func (s *Service) obtainHardwareDetailsRAM(sshClient sshclient.Client) (int, error) {
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
	mebiBytes := kibiBytes / 1024

	return mebiBytes, nil
}

func (s *Service) obtainHardwareDetailsNics(sshClient sshclient.Client) ([]infrav1.NIC, error) {
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

func (s *Service) obtainHardwareDetailsStorage(sshClient sshclient.Client) ([]infrav1.Storage, error) {
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
			return nil, fmt.Errorf("unknown ROTA %s. Expect either 1 or 0", storage.Rota)
		}

		sizeGB := sizeBytes / 1000000000
		capacityGB := infrav1.Capacity(sizeGB)

		if storage.Type == "disk" {
			storageArray = append(storageArray, infrav1.Storage{
				Name:         storage.Name,
				SizeBytes:    infrav1.Capacity(sizeBytes),
				SizeGB:       capacityGB,
				Vendor:       storage.Vendor,
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

func (s *Service) obtainHardwareDetailsCPU(sshClient sshclient.Client) (cpu infrav1.CPU, err error) {
	out := sshClient.GetHardwareDetailsCPUArch()
	if err := handleSSHError(out); err != nil {
		return infrav1.CPU{}, err
	}
	stdOut := trimLineBreak(out.StdOut)
	if stdOut == "" {
		return infrav1.CPU{}, sshclient.ErrEmptyStdOut
	}

	cpu.Arch = stdOut

	out = sshClient.GetHardwareDetailsCPUModel()
	stdOut = trimLineBreak(out.StdOut)
	if err := handleSSHError(out); err != nil {
		return infrav1.CPU{}, err
	}
	if stdOut == "" {
		return infrav1.CPU{}, sshclient.ErrEmptyStdOut
	}

	cpu.Model = stdOut

	out = sshClient.GetHardwareDetailsCPUClockGigahertz()
	stdOut = trimLineBreak(out.StdOut)
	if err := handleSSHError(out); err != nil {
		return infrav1.CPU{}, err
	}
	if stdOut == "" {
		return infrav1.CPU{}, sshclient.ErrEmptyStdOut
	}

	cpu.ClockGigahertz = infrav1.ClockSpeed(stdOut)

	out = sshClient.GetHardwareDetailsCPUThreads()
	stdOut = trimLineBreak(out.StdOut)
	if err := handleSSHError(out); err != nil {
		return infrav1.CPU{}, err
	}
	if stdOut == "" {
		return infrav1.CPU{}, sshclient.ErrEmptyStdOut
	}

	threads, err := strconv.Atoi(stdOut)
	if err != nil {
		return infrav1.CPU{}, fmt.Errorf("failed to parse string to int. Stdout %s: %w", stdOut, err)
	}
	cpu.Threads = threads

	out = sshClient.GetHardwareDetailsCPUFlags()
	if err := handleSSHError(out); err != nil {
		return infrav1.CPU{}, err
	}
	if stdOut == "" {
		return infrav1.CPU{}, sshclient.ErrEmptyStdOut
	}

	flags := strings.Split(stdOut, " ")
	cpu.Flags = flags

	return cpu, err
}

func handleSSHError(out sshclient.Output) error {
	if out.Err != nil {
		return fmt.Errorf("failed to perform ssh command: %w", out.Err)
	}
	if out.StdErr != "" {
		return fmt.Errorf("error occurred during ssh command. StdErr: %s", out.StdErr)
	}
	return nil
}

func (s *Service) actionImageInstalling() actionResult {
	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	// Ensure os ssh secret
	if s.scope.OSSSHSecret == nil {
		return s.recordActionFailure(infrav1.PreparationError, infrav1.ErrorMessageMissingOSSSHSecret)
	}
	sshKey, actResult := s.ensureSSHKey(s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef, s.scope.OSSSHSecret)
	if _, complete := actResult.(actionComplete); !complete {
		return actResult
	}

	s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.OSKey = &sshKey

	image := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.Image
	imagePath, needsDownload, errorMessage := image.GetDetails()
	if errorMessage != "" {
		return s.recordActionFailure(infrav1.ProvisioningError, errorMessage)
	}
	if needsDownload {
		out := sshClient.DownloadImage(imagePath, image.URL)
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to download image: %w", err)}
		}
	}

	// Get device name from storage device
	storageDevices, err := s.obtainHardwareDetailsStorage(sshClient)
	if err != nil {
		return actionError{err: err}
	}

	deviceNames := getDeviceNames(s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.ListOfWWN(), storageDevices)

	// Should find a storage device
	if len(deviceNames) == 0 {
		return s.recordActionFailure(infrav1.ProvisioningError, "no suitable storage device found")
	}

	hostName := infrav1.BareMetalHostNamePrefix + s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name
	// Create autosetup file
	autoSetupInput := autoSetupInput{
		osDevices: deviceNames,
		hostName:  hostName,
		image:     imagePath,
	}

	autoSetup := buildAutoSetup(*s.scope.HetznerBareMetalHost.Spec.Status.InstallImage, autoSetupInput)

	out := sshClient.CreateAutoSetup(autoSetup)
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to create autosetup %s: %w", autoSetup, err)}
	}

	// Create post install script
	postInstallScript := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.PostInstallScript

	if postInstallScript != "" {
		out := sshClient.CreatePostInstallScript(postInstallScript)
		if err := handleSSHError(out); err != nil {
			return actionError{err: fmt.Errorf("failed to create post install script %s: %w", postInstallScript, err)}
		}
	}

	// Execute install image
	out = sshClient.ExecuteInstallImage(postInstallScript != "")
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to execute installimage: %w", err)}
	}

	// Update name in robot API
	if _, err := s.scope.RobotClient.SetBMServerName(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		infrav1.BareMetalHostNamePrefix+s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name,
	); err != nil {
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function SetBMServerName",
			)
			return actionContinue{}
		}
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	out = sshClient.Reboot()
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to reboot server: %w", err)}
	}

	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func getDeviceNames(wwn []string, storageDevices []infrav1.Storage) []string {
	deviceNames := make([]string, 0, len(storageDevices))
	for _, device := range storageDevices {
		if stringInList(device.WWN, wwn) {
			deviceNames = append(deviceNames, device.Name)
		}
	}
	return deviceNames
}

func stringInList(str string, list []string) bool {
	for _, s := range list {
		if str == s {
			return true
		}
	}
	return false
}

func (s *Service) actionProvisioning() actionResult {
	port := s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage
	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})

	// Check hostname with sshClient
	out := sshClient.GetHostName()
	if trimLineBreak(out.StdOut) != infrav1.BareMetalHostNamePrefix+s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name {
		creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
		in := sshclient.Input{
			PrivateKey: creds.PrivateKey,
			Port:       rescuePort,
			IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
		}
		rescueSSHClient := s.scope.SSHClientFactory.NewClient(in)

		isTimeout, isConnectionFailed, err := handleIncompleteBootInstallImage(out, rescueSSHClient, port)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - installImage: %w", err)}
		}
		if err := s.handleIncompleteBootError(false, isTimeout, isConnectionFailed); err != nil {
			return actionError{err: fmt.Errorf("failed to handle incomplete boot: %w", err)}
		}
		return actionContinue{delay: 10 * time.Second}
	}

	out = sshClient.EnsureCloudInit()
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to ensure cloud init: %w", err)}
	}

	if trimLineBreak(out.StdOut) == "" {
		return s.recordActionFailure(infrav1.ProvisioningError, "cloud init not installed")
	}

	out = sshClient.CreateNoCloudDirectory()
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to create no cloud directory: %w", err)}
	}

	out = sshClient.CreateMetaData(infrav1.BareMetalHostNamePrefix + s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name)
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to create meta data: %w", err)}
	}

	userData, err := s.scope.GetRawBootstrapData(context.TODO())
	if err != nil {
		return actionError{err: fmt.Errorf("failed to get user data: %w", err)}
	}

	out = sshClient.CreateUserData(string(userData))
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to create user data: %w", err)}
	}

	out = sshClient.Reboot()
	if err := handleSSHError(out); err != nil {
		return actionError{err: fmt.Errorf("failed to reboot: %w", err)}
	}

	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
}

func handleIncompleteBootInstallImage(out sshclient.Output, sshClient sshclient.Client, port int) (isTimeout bool, isConnectionRefused bool, reterr error) {
	// check err
	if out.Err != nil {
		switch {
		case os.IsTimeout(out.Err) || sshclient.IsTimeoutError(out.Err):
			isTimeout = true
		case sshclient.IsAuthenticationFailedError(out.Err):
			// Check whether we are in the wrong system in the case that rescue and os system might be running on the same port.
			if port == 22 {
				secondaryOut := sshClient.GetHostName()
				if secondaryOut.Err == nil {
					// We are in the wrong system, so return false, false, nil
					return
				}
			}
			reterr = fmt.Errorf("wrong ssh key: %w", out.Err)
		case sshclient.IsConnectionRefusedError(out.Err):
			// Check whether we are in the wrong system in the case that rescue and os system are running on different ports.
			if port != 22 {
				// Check whether we are in the wrong system
				secondaryOut := sshClient.GetHostName()
				if secondaryOut.Err == nil {
					// We are in the wrong system, so return false, false, nil
					return
				}
			}
			isConnectionRefused = true

		default:
			reterr = fmt.Errorf("unhandled ssh error while getting hostname: %w", out.Err)
		}
		return
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		reterr = fmt.Errorf("failed to get host name via ssh. StdErr: %s", out.StdErr)
		return
	}

	// check stdout
	switch trimLineBreak(out.StdOut) {
	case "":
		// Hostname should not be empty. This is unexpected.
		reterr = errors.New("error empty hostname")
	case rescue: // We are in wrong boot, nothing has to be done to trigger reboot
	default:
		// We are in the case that hostName != rescue && StdOut != hostName
		// This is unexpected
		reterr = fmt.Errorf("unexpected hostname %s", trimLineBreak(out.StdOut))
	}
	return isTimeout, isConnectionRefused, reterr
}

func (s *Service) actionEnsureProvisioned() actionResult {
	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.GetIPAddress(),
	})

	// Check hostname with sshClient
	out := sshClient.GetHostName()
	if trimLineBreak(out.StdOut) != infrav1.BareMetalHostNamePrefix+s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name {
		isTimeout, isConnectionFailed, err := handleIncompleteBootProvisioned(out)
		if err != nil {
			return actionError{err: fmt.Errorf("failed to handle incomplete boot - provisioning: %w", err)}
		}
		// A connection failed error could mean that cloud init is still running (if cloudInit introduces a new port)
		if isConnectionFailed &&
			s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage != s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit {
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
		}

		if err := s.handleIncompleteBootError(false, isTimeout, isConnectionFailed); err != nil {
			return actionError{err: fmt.Errorf("failed to handle incomplete boot: %w", err)}
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

	s.scope.HetznerBareMetalHost.ClearError()
	return actionComplete{}
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
		s.scope.HetznerBareMetalHost.SetError(infrav1.ErrorTypeSSHRebootNotStarted, "ssh reboot just triggered")
		return actionContinue{delay: 5 * time.Second}, nil
	case strings.Contains(stdOut, "status: done"):

		s.scope.HetznerBareMetalHost.ClearError()
	case strings.Contains(stdOut, "status: error"):
		record.Event(
			s.scope.HetznerBareMetalHost,
			"FatalError",
			"cloud init returned status error",
		)
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
		return actionContinue{delay: 10 * time.Second}
	}

	return actionComplete{}
}

func handleIncompleteBootProvisioned(out sshclient.Output) (isTimeout bool, isConnectionRefused bool, reterr error) {
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
		return
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		reterr = fmt.Errorf("failed to get host name via ssh. StdErr: %s", out.StdErr)
		return
	}

	// check stdout
	switch trimLineBreak(out.StdOut) {
	case "":
		// Hostname should not be empty. This is unexpected.
		reterr = errors.New("error empty hostname")
	case rescue: // We are in wrong boot, nothing has to be done to trigger reboot
	default:
		reterr = fmt.Errorf("unexpected hostname %s", trimLineBreak(out.StdOut))
	}
	return isTimeout, isConnectionRefused, reterr
}

func (s *Service) actionProvisioned() actionResult {
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
			if trimLineBreak(out.StdOut) == infrav1.BareMetalHostNamePrefix+s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name {
				// Reboot has been successful
				s.scope.HetznerBareMetalHost.Spec.Status.Rebooted = false
				s.scope.HetznerBareMetalHost.ClearRebootAnnotations()

				s.scope.HetznerBareMetalHost.ClearError()
				return actionComplete{}
			}
			// Reboot has been ongoing
			isTimeout, isConnectionFailed, err := handleIncompleteBootProvisioned(out)
			if err != nil {
				return actionError{err: fmt.Errorf("failed to handle incomplete boot - provisioning: %w", err)}
			}
			if err := s.handleIncompleteBootError(false, isTimeout, isConnectionFailed); err != nil {
				return actionError{err: fmt.Errorf("failed to handle incomplete boot: %w", err)}
			}
			return actionContinue{delay: 10 * time.Second}
		}
		// Reboot now
		out := sshClient.Reboot()
		if err := handleSSHError(out); err != nil {
			return actionError{err: err}
		}
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
		if models.IsError(err, models.ErrorCodeRateLimitExceeded) {
			conditions.MarkTrue(s.scope.HetznerBareMetalHost, infrav1.RateLimitExceeded)
			record.Event(s.scope.HetznerBareMetalHost,
				"RateLimitExceeded",
				"exceeded rate limit with calling robot function SetBMServerName",
			)
			return actionContinue{}
		}
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
		if err := handleSSHError(out); err != nil {
			s.scope.Info("Error while reseting kubeadm", "err", err)
		}
	} else {
		s.scope.Info("OS SSH Secret is empty - cannot reset kubeadm")
	}

	s.scope.HetznerBareMetalHost.ClearError()

	return actionComplete{}
}

func (s *Service) actionDeleting() actionResult {
	s.scope.Info("Marked to be deleted", "timestamp", s.scope.HetznerBareMetalHost.DeletionTimestamp)

	if !utils.StringInList(s.scope.HetznerBareMetalHost.Finalizers, infrav1.BareMetalHostFinalizer) {
		s.scope.Info("Ready to be deleted")
		return deleteComplete{}
	}

	s.scope.HetznerBareMetalHost.Finalizers = utils.FilterStringFromList(s.scope.HetznerBareMetalHost.Finalizers, infrav1.BareMetalHostFinalizer)
	if err := s.scope.Client.Update(context.Background(), s.scope.HetznerBareMetalHost); err != nil {
		return actionError{fmt.Errorf("failed to remove finalizer: %w", err)}
	}

	s.scope.Info("Cleanup complete. Removed finalizer", "remaining", s.scope.HetznerBareMetalHost.Finalizers)
	return deleteComplete{}
}
