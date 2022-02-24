package host

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/hrobot-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	hoursBeforeDeletion      time.Duration = 36
	rateLimitTimeOut         time.Duration = 660
	rateLimitTimeOutDeletion time.Duration = 120
	sshTimeOut               time.Duration = 5 * time.Second
	softwareResetTimeout     time.Duration = 5 * time.Minute
	hardwareResetTimeout     time.Duration = 60 * time.Minute
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

	// TODO: Check whether ssh keys changed and if so react according to the initialState

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
	if errType == host.Spec.Status.ErrorType {
		host.Spec.Status.ErrorCount++
	} else {
		// new error - start fresh error count
		host.Spec.Status.ErrorCount = 0
	}
}

func (s *Service) recordActionFailure(errorType infrav1.ErrorType, errorMessage string) actionFailed {
	SetErrorMessage(s.scope.HetznerBareMetalHost, errorType, errorMessage)

	return actionFailed{ErrorType: errorType, errorCount: s.scope.HetznerBareMetalHost.Spec.Status.ErrorCount}
}

func (s *Service) setErrorCondition(ctx context.Context, errType infrav1.ErrorType, message string) error {
	SetErrorMessage(s.scope.HetznerBareMetalHost, errType, message)

	s.scope.Info(
		"adding error message",
		"message", message,
	)

	if err := s.saveHostStatus(ctx); err != nil {
		return errors.Wrap(err, "failed to update error message")
	}
	return nil
}

func (s *Service) saveHostStatus(ctx context.Context) error {
	t := metav1.Now()
	s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated = &t

	if err := s.scope.Client.Status().Update(ctx, s.scope.HetznerBareMetalHost); err != nil {
		s.scope.Error(err, "failed to update status", "host", s.scope.HetznerBareMetalHost)
		return errors.Wrap(err, "failed to update status")
	}
	return nil
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

	hetznerSSHKeys, err := s.scope.RobotClient.ListSSHKeys() // hetznerSSHKeys, err :=
	if err != nil {
		if models.IsError(err, models.ErrorCodeNotFound) {
			return s.recordActionFailure(infrav1.RegistrationError, "no ssh key found")
		}
		return actionError{err: errors.Wrap(err, "failed to list ssh heys")}
	}
	// TODO: check whether SSH keys for machine are valid
	foundSSHKey := false
	var sshKey infrav1.SSHKey
	for _, hetznerSSHKey := range hetznerSSHKeys {
		if s.scope.HetznerCluster.Spec.SSHKeys.Robot.Key.Name == hetznerSSHKey.Name {
			foundSSHKey = true
			sshKey.Name = hetznerSSHKey.Name
			sshKey.Fingerprint = hetznerSSHKey.Fingerprint
		}
	}

	// Upload SSH key if not found
	if !foundSSHKey {
		publicKey := string(s.scope.SSHSecret.Data[s.scope.HetznerCluster.Spec.SSHKeys.Robot.Key.PublicKey])
		hetznerSSHKey, err := s.scope.RobotClient.SetSSHKey(s.scope.HetznerCluster.Spec.SSHKeys.Robot.Key.Name, publicKey)
		if err != nil {
			return actionError{err: errors.Wrap(err, "failed to set ssh key")}
		}
		sshKey.Name = hetznerSSHKey.Name
		sshKey.Fingerprint = hetznerSSHKey.Fingerprint
	}

	s.scope.HetznerBareMetalHost.Spec.Status.HetznerRobotSSHKey = &sshKey

	// Populate reset methods in status
	if len(s.scope.HetznerBareMetalHost.Spec.Status.ResetTypes) == 0 {
		reset, err := s.scope.RobotClient.GetReset(s.scope.HetznerBareMetalHost.Spec.ServerID)
		if err != nil {
			return actionError{err: errors.Wrap(err, "failed to get reset")}
		}
		var resetTypes []infrav1.ResetType
		b, err := json.Marshal(reset.Type)
		if err != nil {
			return actionError{err: errors.Wrap(err, "failed to marshal")}
		}
		if err := json.Unmarshal(b, &resetTypes); err != nil {
			return actionError{err: errors.Wrap(err, "failed to unmarshal")}
		}
		s.scope.HetznerBareMetalHost.Spec.Status.ResetTypes = resetTypes
	}

	// Start rescue mode and reset server if necessary
	if !server.Rescue {
		return s.recordActionFailure(infrav1.RegistrationError, "rescue system not available for server")
	}

	rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		return actionError{err: errors.Wrap(err, "failed to get boot rescue")}
	}
	if !rescue.Active {
		if _, err := s.scope.RobotClient.SetBootRescue(
			s.scope.HetznerBareMetalHost.Spec.ServerID,
			s.scope.HetznerBareMetalHost.Spec.Status.HetznerRobotSSHKey.Fingerprint,
		); err != nil {
			return actionError{err: errors.Wrap(err, "failed to set boot rescue")}
		}

		var resetType infrav1.ResetType
		if s.scope.HetznerBareMetalHost.HasSoftwareReset() {
			resetType = infrav1.ResetTypeSoftware
		} else if s.scope.HetznerBareMetalHost.HasHardwareReset() {
			resetType = infrav1.ResetTypeHardware
		} else {
			return actionError{err: errors.New("no software or hardware reset available for host")}
		}

		if _, err := s.scope.RobotClient.ResetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, resetType); err != nil {
			return actionError{err: errors.Wrap(err, "failed to reset bare metal server")}
		}
	}

	s.scope.SetOperationalStatus(infrav1.OperationalStatusDiscovered)
	s.scope.SetErrorCount(0)
	clearError(s.scope.HetznerBareMetalHost)
	return actionComplete{}
}

func (s *Service) actionEnsureRescue(ctx context.Context, info *reconcileInfo) actionResult {
	// Try accessing server with ssh
	isInRescue, isInOS, isTimeout, err := checkHostNameOutput(s.scope.SSHClient.GetHostName())
	if err != nil {
		return actionError{err: errors.Wrap(err, "failed to get host name via ssh")}
	}
	if isInRescue {
		s.scope.SetErrorCount(0)
		clearError(s.scope.HetznerBareMetalHost)
		return actionComplete{}
	}

	// Check whether there has been an error message already, meaning that the reboot did not finish in time
	var emptyErrorType infrav1.ErrorType
	switch s.scope.HetznerBareMetalHost.Spec.Status.ErrorType {
	case emptyErrorType:
		if isTimeout {
			// Reset was too slow - set error message
			var errorType infrav1.ErrorType
			if s.scope.HetznerBareMetalHost.HasSoftwareReset() {
				errorType = infrav1.ErrorTypeSoftwareResetTooSlow
			} else {
				errorType = infrav1.ErrorTypeHardwareResetTooSlow
			}
			if err := s.setErrorCondition(ctx, errorType, "ssh timeout error - server has not restarted yet"); err != nil {
				return actionError{err: errors.Wrap(err, "failed to set error condition")}
			}
		}

		if isInOS {
			// We are not in rescue system. Set an error message and escalate to hw reset if necessary
			// Switch on rescue mode if it is not switched on already (should not happen)
			var resetType infrav1.ResetType
			var errorType infrav1.ErrorType
			if s.scope.HetznerBareMetalHost.HasSoftwareReset() {
				resetType = infrav1.ResetTypeSoftware
				errorType = infrav1.ErrorTypeSoftwareResetNotStarted
			} else if s.scope.HetznerBareMetalHost.HasHardwareReset() {
				resetType = infrav1.ResetTypeHardware
				errorType = infrav1.ErrorTypeHardwareResetNotStarted
			} else {
				return actionError{err: errors.New("no software or hardware reset available for host")}
			}

			// Set an error to keep track of this failure
			if err := s.setErrorCondition(ctx, errorType, "server is not in rescue mode"); err != nil {
				return actionError{err: errors.Wrap(err, "failed to set error condition")}
			}

			if err := s.ensureRescueModeAndResetServer(resetType); err != nil {
				return actionError{err: errors.Wrap(err, "failed to ensure rescue mode and reset server")}
			}
		}

	case infrav1.ErrorTypeSoftwareResetTooSlow:
		if err := s.handleErrorTypeSoftwareResetTooSlow(ctx); err != nil {
			return actionError{err: errors.Wrap(err, "failed to handle ErrorTypeSoftwareResetTooSlow")}
		}

	case infrav1.ErrorTypeHardwareResetTooSlow:
		if err := s.handleErrorTypeHardwareResetTooSlow(ctx); err != nil {
			return actionError{err: errors.Wrap(err, "failed to handle ErrorTypeHardwareResetTooSlow")}
		}

	case infrav1.ErrorTypeSoftwareResetFailed:
		// This error condition indicates that a hardware reset has been performed already. This has been too slow
		if err := s.setErrorCondition(ctx, infrav1.ErrorTypeHardwareResetTooSlow, "hardware reset is too slow"); err != nil {
			return actionError{err: errors.Wrap(err, "failed to set error condition")}
		}

	case infrav1.ErrorTypeHardwareResetFailed:
		if err := s.handleErrorTypeHardwareResetFailed(ctx); err != nil {
			return actionError{err: errors.Wrap(err, "failed to handle ErrorTypeHardwareResetFailed")}
		}

	case infrav1.ErrorTypeSoftwareResetNotStarted:
		if err := s.handleErrorTypeSoftwareResetNotStarted(ctx, isInOS); err != nil {
			return actionError{err: errors.Wrap(err, "failed to handle ErrorTypeSoftwareResetNotStarted")}
		}

	case infrav1.ErrorTypeHardwareResetNotStarted:
		if err := s.handleErrorTypeHardwareResetNotStarted(ctx, isInOS); err != nil {
			return actionError{err: errors.Wrap(err, "failed to handle ErrorTypeHardwareResetNotStarted")}
		}
	}
	return actionContinue{}
}

func (s *Service) handleErrorTypeSoftwareResetTooSlow(ctx context.Context) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, softwareResetTimeout) {
		if err := s.setErrorCondition(ctx, infrav1.ErrorTypeSoftwareResetFailed, "software reset timed out"); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
		// Perform hardware reset
		if _, err := s.scope.RobotClient.ResetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.ResetTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reset bare metal server")
		}
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareResetTooSlow(ctx context.Context) error {
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		if err := s.setErrorCondition(ctx, infrav1.ErrorTypeHardwareResetFailed, "hardware reset timed out"); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
		// Perform hardware reset
		if _, err := s.scope.RobotClient.ResetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.ResetTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reset bare metal server")
		}
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareResetFailed(ctx context.Context) error {
	// If a hardware reset fails we have no option but to trigger a new one if the timeout has been reached.
	if hasTimedOut(s.scope.HetznerBareMetalHost.Spec.Status.LastUpdated, hardwareResetTimeout) {
		if _, err := s.scope.RobotClient.ResetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, infrav1.ResetTypeHardware); err != nil {
			return errors.Wrap(err, "failed to reset bare metal server")
		}
		if err := s.setErrorCondition(ctx, infrav1.ErrorTypeHardwareResetFailed, "hardware reset failed"); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
	}
	return nil
}

func (s *Service) handleErrorTypeSoftwareResetNotStarted(ctx context.Context, isInOS bool) error {
	// Check whether software reset has not been started again and escalate if not.
	// Otherwise set a new error as the software reset has been slow anyway.
	if isInOS {
		if err := s.ensureRescueModeAndResetServer(infrav1.ResetTypeHardware); err != nil {
			return errors.Wrap(err, "failed to ensure rescue mode and hardware reset server")
		}
		// set an error that hardware reset failed to manage further states. If the hardware reset started successfully
		// Then we will complete this or go to ErrorStateHardwareResetTooSlow as expected.
		if err := s.setErrorCondition(
			ctx,
			infrav1.ErrorTypeHardwareResetFailed,
			"hardware reset triggered after software reset did not start",
		); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
	} else {
		if err := s.setErrorCondition(ctx, infrav1.ErrorTypeSoftwareResetTooSlow, "software reset too slow"); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
	}
	return nil
}

func (s *Service) handleErrorTypeHardwareResetNotStarted(ctx context.Context, isInOS bool) error {
	// Check whether software reset has not been started again and escalate if not.
	// Otherwise set a new error as the software reset has been slow anyway.
	if isInOS {
		if err := s.ensureRescueModeAndResetServer(infrav1.ResetTypeHardware); err != nil {
			return errors.Wrap(err, "failed to ensure rescue mode and hardware reset server")
		}
		if err := s.setErrorCondition(
			ctx,
			infrav1.ErrorTypeHardwareResetFailed,
			"hardware reset failed",
		); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
	} else {
		if err := s.setErrorCondition(ctx, infrav1.ErrorTypeHardwareResetTooSlow, "hardware reset too slow"); err != nil {
			return errors.Wrap(err, "failed to set error condition")
		}
	}
	return nil
}

func hasTimedOut(lastUpdated *metav1.Time, timeout time.Duration) bool {
	now := metav1.Now()
	if lastUpdated.Add(timeout).Before(now.Time) {
		return true
	}
	return false
}

func checkHostNameOutput(out sshclient.Output) (isInRescue bool, isInOS bool, isTimeout bool, err error) {
	// check err
	if out.Err != nil {
		if os.IsTimeout(out.Err) {
			isTimeout = true
			return
		}
		err = errors.Wrap(out.Err, "failed to get host name via ssh")
		return
	}

	// check stderr
	if out.StdErr != "" {
		// This is an unexpected error
		err = fmt.Errorf("failed to get host name via ssh. StdErr: %s", out.StdErr)
		return
	}

	// check stdout
	switch out.StdOut {
	case "rescue":
		// We are in rescue system as expected. Go to next state
		isInRescue = true
	case "":
		// Hostname should not be empty. This is unexpected.
		err = errors.New("error empty hostname")
	default:
		isInOS = true
	}
	return
}

func (s *Service) ensureRescueModeAndResetServer(resetType infrav1.ResetType) error {
	rescue, err := s.scope.RobotClient.GetBootRescue(s.scope.HetznerBareMetalHost.Spec.ServerID)
	if err != nil {
		return errors.Wrap(err, "failed to get bare metal server")
	}
	if !rescue.Active {
		// Rescue system is still not active - activate again
		s.scope.Info("Rescue system not active - activate again")
		if _, err := s.scope.RobotClient.SetBootRescue(
			s.scope.HetznerBareMetalHost.Spec.ServerID,
			s.scope.HetznerBareMetalHost.Spec.Status.HetznerRobotSSHKey.Fingerprint,
		); err != nil {
			return errors.Wrap(err, "failed to set boot rescue")
		}
	}

	if _, err := s.scope.RobotClient.ResetBMServer(s.scope.HetznerBareMetalHost.Spec.ServerID, resetType); err != nil {
		return errors.Wrap(err, "failed to reset bare metal server")
	}
	return nil
}

func (s *Service) actionRegistering(ctx context.Context, info *reconcileInfo) actionResult {
	var hardwareDetails infrav1.HardwareDetails
	mebiBytes, err := s.obtainHardwareDetailsRam()
	if err != nil {
		return actionError{err: err}
	}
	hardwareDetails.RAMMebibytes = mebiBytes

	nics, err := s.obtainHardwareDetailsNics()
	if err != nil {
		return actionError{err: err}
	}
	hardwareDetails.NIC = nics

	storage, err := s.obtainHardwareDetailsStorage()
	if err != nil {
		return actionError{err: err}
	}
	hardwareDetails.Storage = storage

	s.scope.HetznerBareMetalHost.Spec.Status.HardwareDetails = &hardwareDetails

	if s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.WWN == "" {
		return s.recordActionFailure(infrav1.RegistrationError, "no root device hints specified yet")
	}
	for _, st := range storage {
		if s.scope.HetznerBareMetalHost.Spec.RootDeviceHints.WWN == st.WWN {
			return actionContinue{}
		}
	}
	return s.recordActionFailure(infrav1.RegistrationError, "no storage device found with root device hints")
}

func (s *Service) obtainHardwareDetailsRam() (int, error) {
	out := s.scope.SSHClient.GetHardwareDetailsRam()
	if err := handleSSHError(out); err != nil {
		return 0, err
	}

	kibiBytes, err := strconv.Atoi(out.StdOut)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse ssh output to memory int. StdOut: %s", out.StdOut)
	}
	mebiBytes := kibiBytes / 1024

	return mebiBytes, nil
}

func (s *Service) obtainHardwareDetailsNics() ([]infrav1.NIC, error) {
	type originalNic struct {
		Name      string `json:"name,omitempty"`
		Model     string `json:"model,omitempty"`
		MAC       string `json:"mac,omitempty"`
		IP        string `json:"ip,omitempty"`
		SpeedMbps string `json:"speedMbps,omitempty"`
	}

	out := s.scope.SSHClient.GetHardwareDetailsNics()
	if err := handleSSHError(out); err != nil {
		return nil, err
	}
	stringArray := strings.Split(out.StdOut, "\n")
	nicsArray := make([]infrav1.NIC, len(stringArray))

	for i, str := range stringArray {
		validJSONString := validJSONFromSSHOutput(str)

		var nic originalNic
		if err := json.Unmarshal([]byte(validJSONString), &nic); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal %v. Original ssh output: %s", validJSONString, out.StdOut)
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

func (s *Service) obtainHardwareDetailsStorage() ([]infrav1.Storage, error) {
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

	out := s.scope.SSHClient.GetHardwareDetailsStorage()
	if err := handleSSHError(out); err != nil {
		return nil, err
	}
	stringArray := strings.Split(out.StdOut, "\n")
	storageArray := make([]infrav1.Storage, 0, len(stringArray))

	for _, str := range stringArray {
		validJSONString := validJSONFromSSHOutput(str)

		var storage originalStorage
		if err := json.Unmarshal([]byte(validJSONString), &storage); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal %v. Original ssh output: %s", validJSONString, out.StdOut)
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

		if storage.Type == "disk" {
			storageArray = append(storageArray, infrav1.Storage{
				Name:         storage.Name,
				SizeBytes:    infrav1.Capacity(sizeBytes),
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

func validJSONFromSSHOutput(str string) string {
	tempString1 := strings.ReplaceAll(str, `" `, `","`)
	tempString2 := strings.ReplaceAll(tempString1, `="`, `":"`)
	return fmt.Sprintf(`{"%s}`, strings.TrimSpace(tempString2))
}

func handleSSHError(out sshclient.Output) error {
	if out.Err != nil {
		return errors.Wrap(out.Err, "failed to perform ssh command")
	}
	if out.StdErr != "" {
		return fmt.Errorf("error occured during ssh command. StdErr: %s", out.StdErr)
	}
	return nil
}
