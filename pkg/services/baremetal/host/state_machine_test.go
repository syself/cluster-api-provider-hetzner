package host

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/hrobot-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("handleEnsureRescue", func() {

	DescribeTable("Complete successfully",
		func(stderr string, hostErrorType infrav1.ErrorType) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHostName").Return(sshclient.Output{StdOut: "rescue", StdErr: stderr})

			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

			host := bareMetalHost(
				withError(hostErrorType, "", 1, metav1.Now()),
			)
			hsm := newTestHostStateMachine(host, newTestService(host, nil, &sshMock, &robotMock))
			hsm.nextState = infrav1.StateEnsureRescue

			Expect(hsm.handleEnsureRescue(ctx, &reconcileInfo{log: log})).Should(BeAssignableToTypeOf(actionComplete{}))
			Expect(host.Spec.Status.ErrorType).To(Equal(infrav1.ErrorType("")))
			Expect(hsm.nextState).To(Equal(infrav1.StateRegistering))
		},
		Entry("without errorType", "", infrav1.ErrorType("")),
		Entry("with errorType", "", infrav1.ErrorTypeHardwareResetFailed),
	)

	DescribeTable("varying error type and ssh client response - robot client giving all positive results, no timeouts",
		func(
			stdout, stderr string,
			err error,
			hostErrorType infrav1.ErrorType,
			expectedActionResult actionResult,
			expectedHostErrorType infrav1.ErrorType,
		) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHostName").Return(sshclient.Output{StdOut: stdout, StdErr: stderr, Err: err})

			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

			host := bareMetalHost(
				withError(hostErrorType, "", 1, metav1.Now()),
			)
			hsm := newTestHostStateMachine(host, newTestService(host, nil, &sshMock, &robotMock))

			Expect(hsm.handleEnsureRescue(ctx, &reconcileInfo{log: log})).Should(BeAssignableToTypeOf(expectedActionResult))
			Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
		},
		Entry("hostName == rescue, error is set", "rescue", "", errors.New("testerror"), infrav1.ErrorType(""), actionError{}, infrav1.ErrorType("")),
		Entry("timeout, no errorType", "", "", timeout, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSoftwareResetTooSlow),
		Entry("stderr set", "", "std error", nil, infrav1.ErrorType(""), actionError{}, infrav1.ErrorType("")),
		Entry("stderr and errorType set", "", "std error", nil, infrav1.ErrorTypeSoftwareResetTooSlow, actionError{}, infrav1.ErrorTypeSoftwareResetTooSlow),
		Entry("timeout,ErrorType == ErrorTypeSoftwareResetTooSlow", "", "", timeout, infrav1.ErrorTypeSoftwareResetTooSlow, actionContinue{}, infrav1.ErrorTypeSoftwareResetTooSlow),
		Entry("timeout,ErrorType == ErrorTypeHardwareResetTooSlow", "", "", timeout, infrav1.ErrorTypeHardwareResetTooSlow, actionContinue{}, infrav1.ErrorTypeHardwareResetTooSlow),
		Entry("timeout,ErrorType == ErrorTypeSoftwareResetFailed", "", "", timeout, infrav1.ErrorTypeSoftwareResetFailed, actionContinue{}, infrav1.ErrorTypeHardwareResetTooSlow),
		Entry("timeout,ErrorType == ErrorTypeHardwareResetFailed", "", "", timeout, infrav1.ErrorTypeHardwareResetFailed, actionContinue{}, infrav1.ErrorTypeHardwareResetFailed),
		Entry("timeout,ErrorType == ErrorTypeSoftwareResetNotStarted", "", "", timeout, infrav1.ErrorTypeSoftwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareResetTooSlow),
		Entry("timeout,ErrorType == ErrorTypeHardwareResetNotStarted", "", "", timeout, infrav1.ErrorTypeHardwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareResetTooSlow),
		Entry("hostname != rescue", "fedoramachine", "", nil, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSoftwareResetNotStarted),
		Entry("hostname != rescue, ErrorType == ErrorTypeSoftwareResetNotStarted", "fedoramachine", "", nil, infrav1.ErrorTypeSoftwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareResetFailed),
		Entry("hostname != rescue, ErrorType == ErrorTypeHardwareResetNotStarted", "fedoramachine", "", nil, infrav1.ErrorTypeHardwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareResetFailed),
	)

	// Test with different reset type only software on machine
	DescribeTable("Different reset types",
		func(
			stdout string,
			err error,
			resetTypes []infrav1.ResetType,
			hostErrorType infrav1.ErrorType,
			expectedHostErrorType infrav1.ErrorType,
			expectedResetType infrav1.ResetType,
		) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHostName").Return(sshclient.Output{StdOut: stdout, StdErr: "", Err: err})

			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

			host := bareMetalHost(
				withError(hostErrorType, "", 1, metav1.Now()),
				withResetTypes(resetTypes),
			)
			hsm := newTestHostStateMachine(host, newTestService(host, nil, &sshMock, &robotMock))

			Expect(hsm.handleEnsureRescue(ctx, &reconcileInfo{log: log})).Should(BeAssignableToTypeOf(actionContinue{}))
			Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
			if expectedResetType != infrav1.ResetType("") {
				Expect(robotMock.AssertCalled(GinkgoT(), "ResetBMServer", bareMetalHostID, expectedResetType)).To(BeTrue())
			}

		},
		Entry("timeout, no errorType, only hw reset", "", timeout, []infrav1.ResetType{infrav1.ResetTypeHardware}, infrav1.ErrorType(""), infrav1.ErrorTypeHardwareResetTooSlow, infrav1.ResetType("")),
		Entry("hostname != rescue, only hw reset", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeHardware}, infrav1.ErrorType(""), infrav1.ErrorTypeHardwareResetNotStarted, infrav1.ResetTypeHardware),
		Entry("hostname != rescue", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorType(""), infrav1.ErrorTypeSoftwareResetNotStarted, infrav1.ResetTypeSoftware),
		Entry("hostname != rescue, errorType = ErrorTypeSoftwareResetNotStarted", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorTypeSoftwareResetNotStarted, infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetTypeHardware),
		Entry("hostname != rescue, errorType = ErrorTypeHardwareResetNotStarted", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorTypeHardwareResetNotStarted, infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetTypeHardware),
	)

	// Test with reached timeouts
	DescribeTable("Different timeouts",
		func(
			hostErrorType infrav1.ErrorType,
			lastUpdated time.Time,
			expectedHostErrorType infrav1.ErrorType,
			expectedResetType infrav1.ResetType,
		) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHostName").Return(sshclient.Output{Err: timeout})

			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

			host := bareMetalHost(
				withError(hostErrorType, "", 1, metav1.Time{Time: lastUpdated}),
			)
			hsm := newTestHostStateMachine(host, newTestService(host, nil, &sshMock, &robotMock))

			Expect(hsm.handleEnsureRescue(ctx, &reconcileInfo{log: log})).Should(BeAssignableToTypeOf(actionContinue{}))
			Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
			if expectedResetType != infrav1.ResetType("") {
				Expect(robotMock.AssertCalled(GinkgoT(), "ResetBMServer", bareMetalHostID, expectedResetType)).To(BeTrue())
			}

		},
		Entry("timed out hw reset", infrav1.ErrorTypeHardwareResetTooSlow, time.Now().Add(-time.Hour), infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetTypeHardware),
		Entry("timed out failed hw reset", infrav1.ErrorTypeHardwareResetFailed, time.Now().Add(-time.Hour), infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetTypeHardware),
		Entry("timed out sw reset", infrav1.ErrorTypeSoftwareResetTooSlow, time.Now().Add(-5*time.Minute), infrav1.ErrorTypeSoftwareResetFailed, infrav1.ResetTypeHardware),
		Entry("not timed out hw reset", infrav1.ErrorTypeHardwareResetTooSlow, time.Now().Add(-30*time.Minute), infrav1.ErrorTypeHardwareResetTooSlow, infrav1.ResetType("")),
		Entry("not timed out failed hw reset", infrav1.ErrorTypeHardwareResetFailed, time.Now().Add(-30*time.Minute), infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetType("")),
		Entry("not timed out sw reset", infrav1.ErrorTypeSoftwareResetTooSlow, time.Now().Add(-3*time.Minute), infrav1.ErrorTypeSoftwareResetTooSlow, infrav1.ResetType("")),
	)
})
