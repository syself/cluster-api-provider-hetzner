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
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/hrobot-go/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func (s *Service) Reconcile(ctx context.Context) (_ *ctrl.Result, err error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Reconciling baremetal host", "name", s.scope.HetznerBareMetalHost.Name)

	initialState := s.scope.HetznerBareMetalHost.Spec.Status.ProvisioningState

	oldHost := *s.scope.HetznerBareMetalHost
	hostStateMachine := newHostStateMachine(s.scope.HetznerBareMetalHost, s, &log)
	actResult := hostStateMachine.ReconcileState(ctx)
	result, err := actResult.Result()
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("action %q failed", initialState))
		return &ctrl.Result{Requeue: true}, err
	}

	if !reflect.DeepEqual(oldHost, s.scope.HetznerBareMetalHost) {
		if err := saveHost(ctx, s.scope.Client, s.scope.HetznerBareMetalHost); err != nil {
			return &ctrl.Result{RequeueAfter: 2 * time.Second}, errors.Wrap(err, fmt.Sprintf("failed to save host status after %q", initialState))
		}
	}

	return &result, nil
}

// Delete implements delete method of bare metal hosts.
func (s *Service) Delete(ctx context.Context) (_ *ctrl.Result, err error) {
	return nil, nil
}

// SetErrorMessage updates the ErrorMessage in the host Status struct and increases the ErrorCount.
func SetErrorMessage(host *infrav1.HetznerBareMetalHost, errType infrav1.ErrorType, message string) {
	if errType == host.Spec.Status.ErrorType && message == host.Spec.Status.ErrorMessage {
		host.Spec.Status.ErrorCount++
	} else {
		// new error - start fresh error count
		host.Spec.Status.ErrorCount = 1
	}
	host.Spec.Status.ErrorType = errType
	host.Spec.Status.ErrorMessage = message
}

func (s *Service) recordActionFailure(errorType infrav1.ErrorType, errorMessage string) actionFailed {
	SetErrorMessage(s.scope.HetznerBareMetalHost, errorType, errorMessage)
	s.scope.Error(errors.New("action failure"), errorMessage, "errorType", errorType)
	return actionFailed{ErrorType: errorType, errorCount: s.scope.HetznerBareMetalHost.Spec.Status.ErrorCount}
}

// SetErrorCondition sets the error in host status and updates the host object.
func SetErrorCondition(ctx context.Context, host *infrav1.HetznerBareMetalHost, client client.Client, errType infrav1.ErrorType, message string) error {
	SetErrorMessage(host, errType, message)

	if err := saveHost(ctx, client, host); err != nil {
		return errors.Wrap(err, "failed to update error message")
	}
	return nil
}

func saveHost(ctx context.Context, client client.Client, host *infrav1.HetznerBareMetalHost) error {
	t := metav1.Now()
	host.Spec.Status.LastUpdated = &t

	if err := client.Update(ctx, host); err != nil {
		return errors.Wrap(err, "failed to update status")
	}
	return nil
}

// clearError removes any existing error message.
func clearError(host *infrav1.HetznerBareMetalHost) {
	var emptyErrType infrav1.ErrorType
	if host.Spec.Status.ErrorType != emptyErrType {
		host.Spec.Status.ErrorType = emptyErrType
	}
	if host.Spec.Status.ErrorMessage != "" {
		host.Spec.Status.ErrorMessage = ""
	}
}

// hasRebootAnnotation checks for existence of reboot annotations and returns true if at least one exist.
func hasRebootAnnotation(host infrav1.HetznerBareMetalHost) bool {
	for annotation := range host.GetAnnotations() {
		if isRebootAnnotation(annotation) {
			return true
		}
	}
	return false
}

// isRebootAnnotation returns true if the provided annotation is a reboot annotation (either suffixed or not).
func isRebootAnnotation(annotation string) bool {
	return strings.HasPrefix(annotation, infrav1.RebootAnnotation+"/") || annotation == infrav1.RebootAnnotation
}

// clearRebootAnnotations deletes all reboot annotations that exist on the provided host.
func clearRebootAnnotations(host *infrav1.HetznerBareMetalHost) {
	for annotation := range host.Annotations {
		if isRebootAnnotation(annotation) {
			delete(host.Annotations, annotation)
		}
	}
}

func (s *Service) getSSHKeysAndUpdateStatus() (osSSHSecret *corev1.Secret, rescueSSHSecret *corev1.Secret) {
	// Set ssh status if none has been set so far
	osSSHSecret = s.scope.OSSSHSecret
	rescueSSHSecret = s.scope.RescueSSHSecret
	// If os ssh secret is set and the status not yet, then update it
	if s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.CurrentOS == nil && osSSHSecret != nil {
		s.scope.HetznerBareMetalHost.UpdateOSSSHStatus(*osSSHSecret)
	}
	if s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.CurrentRescue == nil {
		s.scope.HetznerBareMetalHost.UpdateRescueSSHStatus(*rescueSSHSecret)
	}
	return osSSHSecret, rescueSSHSecret
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

func (s *Service) actionNone() actionResult {
	server, err := s.scope.RobotClient.GetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		if models.IsError(err, models.ErrorCodeServerNotFound) {
			return s.recordActionFailure(
				infrav1.RegistrationError,
				fmt.Sprintf("bare metal host with id %v not found", s.scope.HetznerBareMetalHost.Spec.ServerID),
			)
		}
		return actionError{err: errors.Wrap(err, "failed to get bare metal server")}
	}

	s.scope.HetznerBareMetalHost.Spec.Status.IP = server.ServerIP

	sshKey, actResult := s.ensureSSHKey(s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef, s.scope.RescueSSHSecret)
	if _, complete := actResult.(actionComplete); !complete {
		return actResult
	}

	s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey = &sshKey

	// Populate reboot methods in status
	if len(s.scope.HetznerBareMetalHost.Spec.Status.RebootTypes) == 0 {
		reboot, err := s.scope.RobotClient.GetReboot(s.scope.HetznerBareMetalHost.Spec.ServerID)
		if err != nil {
			return actionError{err: errors.Wrap(err, "failed to get reboot")}
		}
		var rebootTypes []infrav1.RebootType
		b, err := json.Marshal(reboot.Type)
		if err != nil {
			return actionError{err: errors.Wrap(err, "failed to marshal")}
		}
		if err := json.Unmarshal(b, &rebootTypes); err != nil {
			return actionError{err: errors.Wrap(err, "failed to unmarshal")}
		}
		s.scope.HetznerBareMetalHost.Spec.Status.RebootTypes = rebootTypes
	}

	// Start rescue mode and reboot server if necessary
	if !server.Rescue {
		return s.recordActionFailure(infrav1.RegistrationError, "rescue system not available for server")
	}

	// Delete old rescue activations if exist, as the ssh key might have changed in between
	if _, err := s.scope.RobotClient.DeleteBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID); err != nil {
		return actionError{err: errors.Wrap(err, "failed to delete boot rescue")}
	}

	if _, err := s.scope.RobotClient.SetBootRescue(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey.Fingerprint,
	); err != nil {
		return actionError{err: errors.Wrap(err, "failed to set boot rescue")}
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
		return actionError{err: errors.Wrap(err, "failed to reboot bare metal server")}
	}

	s.scope.SetErrorCount(0)
	clearError(s.scope.HetznerBareMetalHost)
	return actionComplete{}
}

func (s *Service) ensureSSHKey(sshSecretRef infrav1.SSHSecretRef, sshSecret *corev1.Secret) (infrav1.SSHKey, actionResult) {
	hetznerSSHKeys, err := s.scope.RobotClient.ListSSHKeys()
	if err != nil {
		if !models.IsError(err, models.ErrorCodeNotFound) {
			return infrav1.SSHKey{}, actionError{err: errors.Wrap(err, "failed to list ssh heys")}
		}
	}

	foundSSHKey := false
	var sshKey infrav1.SSHKey
	for _, hetznerSSHKey := range hetznerSSHKeys {
		if string(sshSecret.Data[sshSecretRef.Key.Name]) == hetznerSSHKey.Name {
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
			return infrav1.SSHKey{}, actionError{err: errors.Wrap(err, "failed to set ssh key")}
		}
		sshKey.Name = hetznerSSHKey.Name
		sshKey.Fingerprint = hetznerSSHKey.Fingerprint
	}
	return sshKey, actionComplete{}
}

func (s *Service) actionEnsureCorrectBoot(hostName string, osSSHPort *int) actionResult {
	if hostName != rescue && osSSHPort == nil {
		return actionError{err: errors.New("cannot procceed without osSSHPort specified")}
	}

	mainSSHClient := s.getMainSSHClient(hostName, osSSHPort)
	secondarySSHClient := s.getSecondarySSHClient(hostName, osSSHPort)
	// Try accessing server with ssh
	isInCorrectBoot, isTimeout, isConnectionRefused, err := checkHostNameOutput(mainSSHClient.GetHostName(), secondarySSHClient, hostName, osSSHPort)
	if err != nil {
		return actionError{err: errors.Wrap(err, "failed to get host name via ssh")}
	}
	if isInCorrectBoot {
		s.scope.SetErrorCount(0)
		clearError(s.scope.HetznerBareMetalHost)
		return actionComplete{}
	}
	if isConnectionRefused {
		if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
			if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, time.Minute) {
				return actionError{err: errors.New("connection refused error of ssh. Might be due to wrong port")}
			}
		} else {
			SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeConnectionError, "ssh gave connection error")
		}
		return actionContinue{delay: 5 * time.Second}
	} else if s.scope.HetznerBareMetalHost.Spec.Status.ErrorType == infrav1.ErrorTypeConnectionError {
		// Remove error if it is of type connection error
		clearError(s.scope.HetznerBareMetalHost)
		s.scope.SetErrorCount(0)
	}
	// Check whether there has been an error message already, meaning that the reboot did not finish in time
	var emptyErrorType infrav1.ErrorType
	switch s.scope.HetznerBareMetalHost.Spec.Status.ErrorType {
	case emptyErrorType:
		if isTimeout {
			// Reset was too slow - set error message
			SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSSHRebootTooSlow, "ssh timeout error - server has not restarted yet")
			return actionContinue{delay: 5 * time.Second}
		}

		// We are not in correct boot. Trigger SSH reboot.
		if hostName == rescue {
			if err := s.ensureRescueMode(); err != nil {
				return actionError{err: errors.Wrap(err, "failed to ensure correct boot mode and reboot server")}
			}
		}

		// Perform reboot
		actResult := s.performReboot(secondarySSHClient)
		if _, complete := actResult.(actionComplete); !complete {
			return actResult
		}

	case infrav1.ErrorTypeSSHRebootTooSlow:
		err = s.handleErrorTypeSSHRebootTooSlow(isTimeout)

	case infrav1.ErrorTypeSoftwareRebootTooSlow:
		err = s.handleErrorTypeSoftwareRebootTooSlow(isTimeout)

	case infrav1.ErrorTypeHardwareRebootTooSlow:
		err = s.handleErrorTypeHardwareRebootTooSlow(isTimeout)

	case infrav1.ErrorTypeHardwareRebootFailed:
		err = s.handleErrorTypeHardwareRebootFailed()

	case infrav1.ErrorTypeSSHRebootNotStarted:
		err = s.handleErrorTypeSSHRebootNotStarted(!isTimeout, hostName == rescue)

	case infrav1.ErrorTypeSoftwareRebootNotStarted:
		err = s.handleErrorTypeSoftwareRebootNotStarted(!isTimeout, hostName == rescue)

	case infrav1.ErrorTypeHardwareRebootNotStarted:
		err = s.handleErrorTypeHardwareRebootNotStarted(!isTimeout, hostName == rescue)
	}
	if err != nil {
		return actionError{err: err}
	}
	return actionContinue{delay: 5 * time.Second}
}

func (s *Service) getMainSSHClient(hostName string, osSSHPort *int) sshclient.Client {
	var port int
	var privateSSHKey string

	if hostName == rescue {
		port = rescuePort
		privateSSHKey = sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef).PrivateKey
	} else {
		port = *osSSHPort
		privateSSHKey = sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey
	}

	in := sshclient.Input{
		PrivateKey: privateSSHKey,
		Port:       port,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
	}
	return s.scope.SSHClientFactory.NewClient(in)
}

func (s *Service) getSecondarySSHClient(hostName string, osSSHPort *int) sshclient.Client {
	var port int
	var privateSSHKey string

	if osSSHPort != nil {
		if hostName == rescue {
			port = *osSSHPort
			privateSSHKey = sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey
		} else {
			port = rescuePort
			privateSSHKey = sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef).PrivateKey
		}

		in := sshclient.Input{
			PrivateKey: privateSSHKey,
			Port:       port,
			IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
		}
		return s.scope.SSHClientFactory.NewClient(in)
	}

	// If no osSSHPort is specified, then we don't need this client
	return nil
}

func (s *Service) performReboot(secondarySSHClient sshclient.Client) actionResult {
	// Check whether we should try ssh reboot
	if secondarySSHClient != nil {
		out := secondarySSHClient.Reboot()
		if err := handleSSHError(out); err == nil {
			// SSH reboot has been performed successfully. Despite that
			// Set error message that ssh reboot did not start in order to keep track of this error.
			SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSSHRebootNotStarted, "ssh reboot just triggered")
			return actionContinue{delay: 5 * time.Second}
		}
	}
	// Perform software or hardware reboot if ssh reboot failed or is not available
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
		return s.recordActionFailure(infrav1.PreparationError, "no software or hardware reboot available for host")
	}

	if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
		return actionError{err: errors.Wrap(err, "failed to reboot bare metal server")}
	}
	// Set error message that ssh reboot did not start in order to keep track of this error.
	SetErrorMessage(s.scope.HetznerBareMetalHost, errorType, "software/hardeware reboot just triggered")

	return actionComplete{}
}

func (s *Service) handleErrorTypeSSHRebootTooSlow(isTimeout bool) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, sshResetTimeout) {
		// Perform software or hardware reboot
		var rebootType infrav1.RebootType
		var errorType infrav1.ErrorType
		switch {
		case s.scope.HetznerBareMetalHost.HasSoftwareReboot():
			rebootType = infrav1.RebootTypeSoftware
			errorType = infrav1.ErrorTypeSoftwareRebootTooSlow
		case s.scope.HetznerBareMetalHost.HasHardwareReboot():
			rebootType = infrav1.RebootTypeHardware
			errorType = infrav1.ErrorTypeHardwareRebootTooSlow
		default:
			return errors.New("no software or hardware reboot available for host")
		}

		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, rebootType); err != nil {
			return errors.Wrap(err, "failed to reboot bare metal server")
		}
		// Set error message that software reboot is too slow as we perform this reboot now
		SetErrorMessage(s.scope.HetznerBareMetalHost, errorType, "ssh reboot timed out")
	}
	if !isTimeout {
		// If it is not a timeout error, then it means we are in the wrong system - not that the reboot is too slow
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSSHRebootNotStarted, "ssh reboot not timed out - wrong boot")
	}
	return nil
}

func (s *Service) handleErrorTypeSoftwareRebootTooSlow(isTimeout bool) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, softwareResetTimeout) {
		// Perform hardware reboot
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reboot bare metal server")
		}
		// Set error message that hardware reboot is too slow as we perform this reboot now
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootTooSlow, "software reboot timed out")
	}
	if !isTimeout {
		// If it is not a timeout error, then it means we are in the wrong system - not that the reboot is too slow
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSoftwareRebootNotStarted, "software reboot not timed out - wrong boot")
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareRebootTooSlow(isTimeout bool) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootFailed, "hardware reboot timed out")
		// Perform hardware reboot
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reboot bare metal server")
		}
	}
	if !isTimeout {
		// If it is not a timeout error, then it means we are in the wrong system - not that the reboot is too slow
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootNotStarted, "hardware reboot not timed out - wrong boot")
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareRebootFailed() error {
	// If a hardware reboot fails we have no option but to trigger a new one if the timeout has been reached.
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reboot bare metal server")
		}
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootFailed, "hardware reboot failed")
	}
	return nil
}

func (s *Service) handleErrorTypeSSHRebootNotStarted(isInWrongBoot bool, wantsRescue bool) error {
	// Check whether ssh reboot has not been started again and escalate if not.
	// Otherwise set a new error as the ssh reboot has just been slow.
	if isInWrongBoot {
		if wantsRescue {
			if err := s.ensureRescueMode(); err != nil {
				return errors.Wrap(err, "failed to ensure rescue mode")
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
			return errors.Wrap(err, "failed to reboot bare metal server")
		}

		// set an error that software reboot failed to manage further states. If the software reboot started successfully
		// Then we will complete this or go to ErrorStateSoftwareResetTooSlow as expected.
		SetErrorMessage(s.scope.HetznerBareMetalHost, errorType, "software/hardware reboot triggered after ssh reboot did not start")
	} else {
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSSHRebootTooSlow, "ssh reboot too slow")
	}
	return nil
}

func (s *Service) handleErrorTypeSoftwareRebootNotStarted(isInWrongBoot bool, wantsRescue bool) error {
	// Check whether software reboot has not been started again and escalate if not.
	// Otherwise set a new error as the software reboot has been slow anyway.
	if isInWrongBoot {
		if wantsRescue {
			if err := s.ensureRescueMode(); err != nil {
				return errors.Wrap(err, "failed to ensure rescue mode")
			}
		}
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reboot bare metal server")
		}

		// set an error that hardware reboot not started to manage further states. If the hardware reboot started successfully
		// Then we will complete this or go to ErrorStateHardwareResetTooSlow as expected.
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootNotStarted, "hardware reboot triggered after software reboot did not start")
	} else {
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSoftwareRebootTooSlow, "software reboot too slow")
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareRebootNotStarted(isInWrongBoot bool, wantsRescue bool) error {
	// Check whether software reboot has not been started again and escalate if not.
	// Otherwise set a new error as the software reboot has been slow anyway.
	if isInWrongBoot {
		if wantsRescue {
			if err := s.ensureRescueMode(); err != nil {
				return errors.Wrap(err, "failed to ensure rescue mode")
			}
		}
		if _, err := s.scope.RobotClient.RebootBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.RebootTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reboot bare metal server")
		}
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootNotStarted, "hardware reboot not started")
	} else {
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeHardwareRebootTooSlow, "hardware reboot too slow")
	}
	return nil
}

func hasTimedOut(lastUpdated *metav1.Time, timeout time.Duration) bool {
	now := metav1.Now()
	return lastUpdated.Add(timeout).Before(now.Time)
}

func checkHostNameOutput(out sshclient.Output, secondarySSHClient sshclient.Client, hostName string, osSSHPort *int) (isInCorrectBoot bool, isTimeout bool, isConnectionRefused bool, err error) {
	// check err
	if out.Err != nil {
		switch {
		case os.IsTimeout(out.Err) || sshclient.IsTimeoutError(out.Err):
			isTimeout = true
		case sshclient.IsAuthenticationFailedError(out.Err):
			// Check whether we are in the wrong system in the case that rescue and os system might be running on the same port.
			if osSSHPort != nil && *osSSHPort == 22 {
				secondaryOut := secondarySSHClient.GetHostName()
				if secondaryOut.Err == nil {
					// We are in the wrong system, so return false, false, nil
					return
				}
			}
			err = errors.Wrap(out.Err, "wrong ssh key")
		case sshclient.IsConnectionRefusedError(out.Err):
			// Check whether we are in the wrong system in the case that rescue and os system are running on different ports.
			if osSSHPort != nil && *osSSHPort != 22 {
				// Check whether we are in the wrong system
				secondaryOut := secondarySSHClient.GetHostName()
				if secondaryOut.Err == nil {
					// We are in the wrong system, so return false, false, nil
					return
				}
			}
			isConnectionRefused = true

		default:
			err = errors.Wrap(out.Err, "unhandled ssh error while getting hostname")
		}
		return
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		err = fmt.Errorf("failed to get host name via ssh. StdErr: %s", out.StdErr)
		return
	}

	// check stdout
	switch trimLineBreak(out.StdOut) {
	case hostName:
		// We are in rescue system as expected. Go to next state
		isInCorrectBoot = true
	case "":
		// Hostname should not be empty. This is unexpected.
		err = errors.New("error empty hostname")
	case rescue:
	default:
		// We are in the case that hostName != rescue && StdOut != hostName
		// This is unexpected
		if hostName != rescue {
			err = fmt.Errorf("unexpected hostname %s. Want %s or rescue", trimLineBreak(out.StdOut), hostName)
		}
	}
	return isInCorrectBoot, isTimeout, isConnectionRefused, err
}

func (s *Service) ensureRescueMode() error {
	rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		return errors.Wrap(err, "failed to get bare metal server")
	}
	if !rescue.Active {
		// Rescue system is still not active - activate again
		s.scope.Info("Rescue system not active - activate again")
		if _, err := s.scope.RobotClient.SetBootRescue(
			s.scope.HetznerBareMetalHost.Spec.ServerID,
			s.scope.HetznerBareMetalHost.Spec.Status.SSHStatus.RescueKey.Fingerprint,
		); err != nil {
			return errors.Wrap(err, "failed to set boot rescue")
		}
	}
	return nil
}

func (s *Service) actionRegistering() actionResult {
	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	if s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails == nil {
		var hardwareDetails infrav1.HardwareDetails

		mebiBytes, err := s.obtainHardwareDetailsRAM(sshClient)
		if err != nil {
			return actionError{err: err}
		}
		hardwareDetails.RAMMebibytes = mebiBytes

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
		s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.WWN == "" {
		return s.recordActionFailure(infrav1.RegistrationError, infrav1.ErrorMessageMissingRootDeviceHints)
	}
	for _, st := range s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails.Storage {
		if s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.WWN == st.WWN {
			s.scope.SetErrorCount(0)
			clearError(s.scope.HetznerBareMetalHost)
			return actionComplete{}
		}
	}
	return s.recordActionFailure(infrav1.RegistrationError, "no storage device found with root device hints")
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
		return 0, errors.Wrapf(err, "failed to parse ssh output to memory int. StdOut: %s", stdOut)
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
			return nil, errors.Wrapf(err, "failed to unmarshal %v. Original ssh output: %s", validJSONString, stdOut)
		}
		speedMbps, err := strconv.Atoi(nic.SpeedMbps)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse int from string %s", nic.SpeedMbps)
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
			return nil, errors.Wrapf(err, "failed to unmarshal %v. Original ssh output: %s", validJSONString, stdOut)
		}
		sizeBytes, err := strconv.Atoi(storage.SizeBytes)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse int from string %s", storage.SizeBytes)
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
		return infrav1.CPU{}, errors.Wrapf(err, "failed to parse string to int. Stdout: %s", stdOut)
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
		return errors.Wrap(out.Err, "failed to perform ssh command")
	}
	if out.StdErr != "" {
		return fmt.Errorf("error occurred during ssh command. StdErr: %s", out.StdErr)
	}
	return nil
}

func (s *Service) actionAvailable() actionResult {
	if s.scope.HetznerBareMetalHost.NeedsProvisioning() {
		return actionComplete{}
	}
	return actionContinue{delay: 10 * time.Second}
}

func (s *Service) actionImageInstalling() actionResult {
	creds := sshclient.CredentialsFromSecret(s.scope.RescueSSHSecret, s.scope.HetznerCluster.Spec.SSHKeys.RobotRescueSecretRef)
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       rescuePort,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
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
	imagePath, needsDownload, errorMessage := getImageDetails(image)
	if errorMessage != "" {
		return s.recordActionFailure(infrav1.ProvisioningError, errorMessage)
	}
	if needsDownload {
		out := sshClient.DownloadImage(imagePath, image.URL)
		if err := handleSSHError(out); err != nil {
			return actionError{err: errors.Wrap(err, "failed to download image")}
		}
	}

	// Get device name from storage device
	storageDevices, err := s.obtainHardwareDetailsStorage(sshClient)
	if err != nil {
		return actionError{err: err}
	}

	var deviceName string
	for _, device := range storageDevices {
		if device.WWN == s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.WWN {
			deviceName = device.Name
		}
	}

	// Should find a storage device
	if deviceName == "" {
		return s.recordActionFailure(infrav1.ProvisioningError, "no suitable storage device found")
	}

	hostName := infrav1.BareMetalHostNamePrefix + s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name
	// Create autosetup file
	autoSetupInput := autoSetupInput{
		osDevice: deviceName,
		hostName: hostName,
		image:    imagePath,
	}

	autoSetup := buildAutoSetup(*s.scope.HetznerBareMetalHost.Spec.Status.InstallImage, autoSetupInput)

	out := sshClient.CreateAutoSetup(autoSetup)
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrapf(err, "failed to create autosetup %s", autoSetup)}
	}

	// Create post install script
	postInstallScript := s.scope.HetznerBareMetalHost.Spec.Status.InstallImage.PostInstallScript

	if postInstallScript != "" {
		out := sshClient.CreatePostInstallScript(postInstallScript)
		if err := handleSSHError(out); err != nil {
			return actionError{err: errors.Wrapf(err, "failed to create post install script %s", postInstallScript)}
		}
	}

	// Execute install image
	out = sshClient.ExecuteInstallImage(postInstallScript != "")
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to execute installimage")}
	}

	// Update name in robot API
	if _, err := s.scope.RobotClient.SetBMServerName(
		s.scope.HetznerBareMetalHost.Spec.ServerID,
		infrav1.BareMetalHostNamePrefix+s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name,
	); err != nil {
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	out = sshClient.Reboot()
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to reboot server")}
	}

	s.scope.SetErrorCount(0)
	clearError(s.scope.HetznerBareMetalHost)
	return actionComplete{}
}

func getImageDetails(image infrav1.Image) (imagePath string, needsDownload bool, errorMessage string) {
	// If image is set, then the URL is also set and we have to download a remote file
	switch {
	case image.Name != "" && image.URL != "":
		suffix, err := infrav1.GetImageSuffix(image.URL)
		if err != nil {
			errorMessage = "wrong image url suffix"
			return
		}
		imagePath = fmt.Sprintf("/root/%s.%s", image.Name, suffix)
		needsDownload = true
	case image.Path != "":
		// In the other case a local imagePath is specified
		imagePath = image.Path
	default:
		errorMessage = "invalid image - need to specify either name and url or path"
	}
	return imagePath, needsDownload, errorMessage
}

func (s *Service) actionProvisioning() actionResult {
	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
	})

	out := sshClient.EnsureCloudInit()
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to ensure cloud init")}
	}

	if trimLineBreak(out.StdOut) == "" {
		return s.recordActionFailure(infrav1.ProvisioningError, "cloud init not installed")
	}

	out = sshClient.CreateNoCloudDirectory()
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to create no cloud directory")}
	}

	out = sshClient.CreateMetaData(infrav1.BareMetalHostNamePrefix + s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name)
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to create meta data")}
	}

	userData, err := s.scope.GetRawBootstrapData(context.TODO())
	if err != nil {
		return actionError{err: errors.Wrap(err, "failed to get user data")}
	}

	out = sshClient.CreateUserData(string(userData))
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to create user data")}
	}

	out = sshClient.Reboot()
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to reboot")}
	}

	s.scope.SetErrorCount(0)
	clearError(s.scope.HetznerBareMetalHost)
	return actionComplete{}
}

func (s *Service) actionEnsureProvisioned() actionResult {
	sshClient := s.scope.SSHClientFactory.NewClient(sshclient.Input{
		PrivateKey: sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef).PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterInstallImage,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
	})

	out := sshClient.CloudInitStatus()
	if err := handleSSHError(out); err != nil {
		return actionError{err: errors.Wrap(err, "failed to reboot")}
	}
	stdOut := trimLineBreak(out.StdOut)
	switch {
	case strings.Contains(stdOut, "status: running"):
		// Cloud init is still running
		return actionContinue{delay: 5 * time.Second}
	case strings.Contains(stdOut, "status: disabled"):
		// Reboot needs to be triggered again - did not start yet
		out = sshClient.Reboot()
		if err := handleSSHError(out); err != nil {
			return actionError{err: errors.Wrap(err, "failed to reboot")}
		}
		SetErrorMessage(s.scope.HetznerBareMetalHost, infrav1.ErrorTypeSSHRebootNotStarted, "ssh reboot just triggered")
		return actionContinue{delay: 5 * time.Second}
	case strings.Contains(stdOut, "status: done"):
		s.scope.SetErrorCount(0)
		clearError(s.scope.HetznerBareMetalHost)
	default:
		return actionError{err: fmt.Errorf("unknown response from cloud init: %s", stdOut)}
	}

	return actionComplete{}
}

func (s *Service) actionProvisioned() actionResult {
	rebootDesired := hasRebootAnnotation(*s.scope.HetznerBareMetalHost)
	isRebooted := s.scope.HetznerBareMetalHost.Status.Rebooted
	creds := sshclient.CredentialsFromSecret(s.scope.OSSSHSecret, s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.SecretRef)
	port := s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit
	in := sshclient.Input{
		PrivateKey: creds.PrivateKey,
		Port:       s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec.PortAfterCloudInit,
		IP:         s.scope.HetznerBareMetalHost.Spec.Status.IP,
	}
	sshClient := s.scope.SSHClientFactory.NewClient(in)

	if rebootDesired {
		if isRebooted {
			// Reboot has been done already. Check whether it has been successful
			actResult := s.actionEnsureCorrectBoot(infrav1.BareMetalHostNamePrefix+s.scope.HetznerBareMetalHost.Spec.ConsumerRef.Name, &port)
			if _, ok := actResult.(actionComplete); ok {
				// Reboot has been successful
				s.scope.HetznerBareMetalHost.Status.Rebooted = false
				clearRebootAnnotations(s.scope.HetznerBareMetalHost)
			}
			return actResult
		}
		// Reboot now
		out := sshClient.Reboot()
		if err := handleSSHError(out); err != nil {
			return actionError{err: err}
		}
		s.scope.HetznerBareMetalHost.Status.Rebooted = true
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
		return actionError{err: fmt.Errorf("failed to update name of host in robot API: %w", err)}
	}

	// Remove all data related to bare metal machine
	s.scope.HetznerBareMetalHost.Spec.Status.InstallImage = nil
	s.scope.HetznerBareMetalHost.Spec.Status.UserData = nil
	s.scope.HetznerBareMetalHost.Spec.Status.SSHSpec = nil
	s.scope.HetznerBareMetalHost.Spec.ConsumerRef = nil

	s.scope.SetErrorCount(0)
	clearError(s.scope.HetznerBareMetalHost)

	return actionComplete{}
}
