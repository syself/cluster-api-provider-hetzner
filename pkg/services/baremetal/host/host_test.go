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

var _ = Describe("obtainHardwareDetailsNics", func() {
	DescribeTable("Complete successfully",
		func(stdout string, expectedOutput []infrav1.NIC) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHardwareDetailsNics").Return(sshclient.Output{StdOut: stdout})

			host := bareMetalHost()

			service := newTestService(host, nil, &sshMock, nil)

			Expect(service.obtainHardwareDetailsNics()).Should(Equal(expectedOutput))
		},
		Entry(
			"proper response",
			`name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ip="23.88.6.239/26" speedMbps="1000"
	name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ip="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
			[]infrav1.NIC{
				{
					Name:      "eth0",
					Model:     "Realtek Semiconductor Co.",
					MAC:       "a8:a1:59:94:19:42",
					IP:        "23.88.6.239/26",
					SpeedMbps: 1000,
				}, {
					Name:      "eth0",
					Model:     "Realtek Semiconductor Co.",
					MAC:       "a8:a1:59:94:19:42",
					IP:        "2a01:4f8:272:3e0f::2/64",
					SpeedMbps: 1000,
				},
			}),
	)
})

var _ = Describe("obtainHardwareDetailsStorage", func() {
	DescribeTable("Complete successfully",
		func(stdout string, expectedOutput []infrav1.Storage) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHardwareDetailsStorage").Return(sshclient.Output{StdOut: stdout})

			host := bareMetalHost()

			service := newTestService(host, nil, &sshMock, nil)

			Expect(service.obtainHardwareDetailsStorage()).Should(Equal(expectedOutput))
		},
		Entry(
			"proper response",
			`NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			[]infrav1.Storage{
				{
					Name:         "nvme2n1",
					HCTL:         "",
					Model:        "SAMSUNG MZVL22T0HBLB-00B00",
					Vendor:       "",
					SerialNumber: "S677NF0R402742",
					SizeBytes:    2048408248320,
					WWN:          "eui.002538b411b2cee8",
					Rota:         false,
				},
				{
					Name:         "nvme1n1",
					HCTL:         "",
					Model:        "SAMSUNG MZVLB512HAJQ-00000",
					Vendor:       "",
					SerialNumber: "S3W8NX0N811178",
					SizeBytes:    512110190592,
					WWN:          "eui.0025388801b4dff2",
					Rota:         false,
				},
			}),
	)
})

var _ = Describe("handleRegistering", func() {
	Context("correct hostname == rescue", func() {

		DescribeTable("Complete successfully",
			func(stderr string, hostErrorType infrav1.ErrorType) {
				sshMock := sshmock.Client{}
				sshMock.On("GetHostName").Return(sshclient.Output{StdOut: "rescue", StdErr: stderr})
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := bareMetalHost(
					withError(hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, nil, &sshMock, &robotMock)

				Expect(service.actionEnsureCorrectBoot(ctx, &reconcileInfo{log: log}, "rescue")).Should(BeAssignableToTypeOf(actionComplete{}))
				Expect(host.Spec.Status.ErrorType).To(Equal(infrav1.ErrorType("")))
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
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := bareMetalHost(
					withError(hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, nil, &sshMock, &robotMock)

				Expect(service.actionEnsureCorrectBoot(ctx, &reconcileInfo{log: log}, "rescue")).Should(BeAssignableToTypeOf(expectedActionResult))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
			},
			Entry("hostName == rescue, error is set", "rescue", "", errors.New("testerror"), infrav1.ErrorType(""), actionError{}, infrav1.ErrorType("")),
			Entry("timeout, no errorType", "", "", timeout, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHResetTooSlow),
			Entry("stderr set", "", "std error", nil, infrav1.ErrorType(""), actionError{}, infrav1.ErrorType("")),
			Entry("stderr and errorType set", "", "std error", nil, infrav1.ErrorTypeSoftwareResetTooSlow, actionError{}, infrav1.ErrorTypeSoftwareResetTooSlow),
			Entry("timeout,ErrorType == ErrorTypeSoftwareResetTooSlow", "", "", timeout, infrav1.ErrorTypeSoftwareResetTooSlow, actionContinue{}, infrav1.ErrorTypeSoftwareResetTooSlow),
			Entry("timeout,ErrorType == ErrorTypeHardwareResetTooSlow", "", "", timeout, infrav1.ErrorTypeHardwareResetTooSlow, actionContinue{}, infrav1.ErrorTypeHardwareResetTooSlow),
			Entry("timeout,ErrorType == ErrorTypeHardwareResetFailed", "", "", timeout, infrav1.ErrorTypeHardwareResetFailed, actionContinue{}, infrav1.ErrorTypeHardwareResetFailed),
			Entry("timeout,ErrorType == ErrorTypeSoftwareResetNotStarted", "", "", timeout, infrav1.ErrorTypeSoftwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareResetTooSlow),
			Entry("timeout,ErrorType == ErrorTypeHardwareResetNotStarted", "", "", timeout, infrav1.ErrorTypeHardwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareResetTooSlow),
			Entry("hostname != rescue", "fedoramachine", "", nil, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHResetNotStarted),
			Entry("hostname != rescue, ErrorType == ErrorTypeSoftwareResetNotStarted", "fedoramachine", "", nil, infrav1.ErrorTypeSoftwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareResetNotStarted),
			Entry("hostname != rescue, ErrorType == ErrorTypeHardwareResetNotStarted", "fedoramachine", "", nil, infrav1.ErrorTypeHardwareResetNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareResetNotStarted),
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
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := bareMetalHost(
					// Make sure that timeouts are exceeded to trigger escalation step
					withError(hostErrorType, "", 1, metav1.NewTime(time.Now().Add(-time.Hour))),
					withResetTypes(resetTypes),
				)
				service := newTestService(host, nil, &sshMock, &robotMock)

				Expect(service.actionEnsureCorrectBoot(ctx, &reconcileInfo{log: log}, "rescue")).Should(BeAssignableToTypeOf(actionContinue{}))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectedResetType != infrav1.ResetType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "ResetBMServer", bareMetalHostID, expectedResetType)).To(BeTrue())
				}

			},
			Entry("timeout, no errorType, only hw reset", "", timeout, []infrav1.ResetType{infrav1.ResetTypeHardware}, infrav1.ErrorTypeSSHResetTooSlow, infrav1.ErrorTypeHardwareResetTooSlow, infrav1.ResetTypeHardware),
			Entry("hostname != rescue, only hw reset", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeHardware}, infrav1.ErrorType(""), infrav1.ErrorTypeSSHResetNotStarted, infrav1.ResetType("")),
			Entry("hostname != rescue", "", timeout, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorTypeSSHResetTooSlow, infrav1.ErrorTypeSoftwareResetTooSlow, infrav1.ResetTypeSoftware),
			Entry("hostname != rescue, only hw reset", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeHardware}, infrav1.ErrorTypeSSHResetNotStarted, infrav1.ErrorTypeHardwareResetNotStarted, infrav1.ResetTypeHardware),
			Entry("hostname != rescue, only hw reset", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorTypeSSHResetNotStarted, infrav1.ErrorTypeSoftwareResetNotStarted, infrav1.ResetTypeSoftware),
			Entry("hostname != rescue, errorType = ErrorTypeSoftwareResetNotStarted", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorTypeSoftwareResetNotStarted, infrav1.ErrorTypeHardwareResetNotStarted, infrav1.ResetTypeHardware),
			Entry("hostname != rescue, errorType = ErrorTypeHardwareResetNotStarted", "fedoramachine", nil, []infrav1.ResetType{infrav1.ResetTypeSoftware, infrav1.ResetTypeHardware}, infrav1.ErrorTypeHardwareResetNotStarted, infrav1.ErrorTypeHardwareResetNotStarted, infrav1.ResetTypeHardware),
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
				service := newTestService(host, nil, &sshMock, &robotMock)

				Expect(service.actionEnsureCorrectBoot(ctx, &reconcileInfo{log: log}, "rescue")).Should(BeAssignableToTypeOf(actionContinue{}))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectedResetType != infrav1.ResetType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "ResetBMServer", bareMetalHostID, expectedResetType)).To(BeTrue())
				}

			},
			Entry("timed out hw reset", infrav1.ErrorTypeHardwareResetTooSlow, time.Now().Add(-time.Hour), infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetTypeHardware),
			Entry("timed out failed hw reset", infrav1.ErrorTypeHardwareResetFailed, time.Now().Add(-time.Hour), infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetTypeHardware),
			Entry("timed out sw reset", infrav1.ErrorTypeSoftwareResetTooSlow, time.Now().Add(-5*time.Minute), infrav1.ErrorTypeHardwareResetTooSlow, infrav1.ResetTypeHardware),
			Entry("not timed out hw reset", infrav1.ErrorTypeHardwareResetTooSlow, time.Now().Add(-30*time.Minute), infrav1.ErrorTypeHardwareResetTooSlow, infrav1.ResetType("")),
			Entry("not timed out failed hw reset", infrav1.ErrorTypeHardwareResetFailed, time.Now().Add(-30*time.Minute), infrav1.ErrorTypeHardwareResetFailed, infrav1.ResetType("")),
			Entry("not timed out sw reset", infrav1.ErrorTypeSoftwareResetTooSlow, time.Now().Add(-3*time.Minute), infrav1.ErrorTypeSoftwareResetTooSlow, infrav1.ResetType("")),
		)
	})

	Context("hostname rescue vs machinename", func() {

		DescribeTable("vary hostname and see whether rescue gets triggered",
			func(
				stdout, stderr string,
				err error,
				hostName string,
				hostErrorType infrav1.ErrorType,
				expectedActionResult actionResult,
				expectedHostErrorType infrav1.ErrorType,
				expectsRescueCall bool,
			) {
				sshMock := sshmock.Client{}
				sshMock.On("GetHostName").Return(sshclient.Output{StdOut: stdout, StdErr: stderr, Err: err})
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("ResetBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := bareMetalHost(
					withError(hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, nil, &sshMock, &robotMock)

				Expect(service.actionEnsureCorrectBoot(ctx, &reconcileInfo{log: log}, hostName)).Should(BeAssignableToTypeOf(expectedActionResult))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectsRescueCall {
					Expect(robotMock.AssertCalled(GinkgoT(), "GetBootRescue", bareMetalHostID)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "GetBootRescue", bareMetalHostID)).To(BeTrue())
				}

			},
			Entry("hostname == rescue", "fedoramachine", "", nil, "rescue", infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHResetNotStarted, true),
			Entry("hostname == machinename", "rescue", "", nil, "machinename", infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHResetNotStarted, false),
			Entry("ErrType == ErrorTypeSSHResetNotStarted", "fedoramachine", "", nil, "rescue", infrav1.ErrorTypeSSHResetNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareResetNotStarted, true),
			Entry("ErrType == ErrorTypeSSHResetNotStarted", "rescue", "", nil, "machinename", infrav1.ErrorTypeSSHResetNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareResetNotStarted, false),
		)
	})
})
