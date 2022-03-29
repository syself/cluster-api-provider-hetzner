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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	bmmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	"github.com/syself/hrobot-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("SetErrorMessage", func() {
	DescribeTable("SetErrorMessage",
		func(
			errorType infrav1.ErrorType,
			errorMessage string,
			hasErrorInStatus bool,
			expectedErrorCount int,
			expectedErrorType infrav1.ErrorType,
			expectedErrorMessage string,
		) {

			var host *infrav1.HetznerBareMetalHost
			if hasErrorInStatus {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithError(infrav1.PreparationError, "first message", 2, metav1.Now()),
				)
			} else {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
				)
			}

			SetErrorMessage(host, errorType, errorMessage)
			Expect(host.Spec.Status.ErrorCount).To(Equal(expectedErrorCount))
			Expect(host.Spec.Status.ErrorMessage).To(Equal(expectedErrorMessage))
			Expect(host.Spec.Status.ErrorType).To(Equal(expectedErrorType))
		},
		Entry(
			"new error with existing one",
			infrav1.RegistrationError, // errorType infrav1.ErrorType
			"new message",             // errorMessage string
			true,                      // hasErrorInStatus bool
			1,                         // expectedErrorCount int
			infrav1.RegistrationError, //	expectedErrorType
			"new message",             // expectedErrorMessage
		),
		Entry(
			"old error with existing one",
			infrav1.PreparationError, // errorType infrav1.ErrorType
			"first message",          // errorMessage string
			true,                     // hasErrorInStatus bool
			3,                        // expectedErrorCount int
			infrav1.PreparationError, // expectedErrorType
			"first message",          // expectedErrorMessage
		),
		Entry(
			"new error without existing one",
			infrav1.RegistrationError, // errorType infrav1.ErrorType
			"new message",             // errorMessage string
			true,                      // hasErrorInStatus bool
			1,                         // expectedErrorCount int
			infrav1.RegistrationError, //	expectedErrorType
			"new message",             // expectedErrorMessage
		),
	)
})

var _ = Describe("obtainHardwareDetailsNics", func() {
	DescribeTable("Complete successfully",
		func(stdout string, expectedOutput []infrav1.NIC) {
			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsNics").Return(sshclient.Output{StdOut: stdout})

			host := helpers.BareMetalHost("test-host", "default",
				helpers.WithRebootTypes([]infrav1.RebootType{
					infrav1.RebootTypeSoftware,
					infrav1.RebootTypeHardware,
					infrav1.RebootTypePower,
				}),
				helpers.WithSSHSpec(),
				helpers.WithSSHStatus(),
			)

			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			Expect(service.obtainHardwareDetailsNics(sshMock)).Should(Equal(expectedOutput))
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
		func(stdout string, expectedOutput []infrav1.Storage, expectedErrorMessage *string) {
			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsStorage").Return(sshclient.Output{StdOut: stdout})

			host := helpers.BareMetalHost("test-host", "default",
				helpers.WithRebootTypes([]infrav1.RebootType{
					infrav1.RebootTypeSoftware,
					infrav1.RebootTypeHardware,
					infrav1.RebootTypePower,
				}),
				helpers.WithSSHSpec(),
				helpers.WithSSHStatus(),
			)

			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			storageDevices, err := service.obtainHardwareDetailsStorage(sshMock)
			Expect(storageDevices).Should(Equal(expectedOutput))
			if expectedErrorMessage != nil {
				Expect(err.Error()).Should(ContainSubstring(*expectedErrorMessage))
			} else {
				Expect(err).To(Succeed())
			}
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
					SizeGB:       2048,
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
					SizeGB:       512,
					WWN:          "eui.0025388801b4dff2",
					Rota:         false,
				},
			},
			nil,
		),
		Entry(
			"wrong rota",
			`NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="2"
	NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
	NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			nil,
			pointer.String("unknown ROTA"),
		),
	)
})

var _ = Describe("actionEnsureCorrectBoot", func() {
	Context("correct hostname == rescue", func() {

		DescribeTable("Complete successfully",
			func(stderr string, hostErrorType infrav1.ErrorType) {
				sshMock := &sshmock.Client{}
				sshMock.On("GetHostName").Return(sshclient.Output{StdOut: "rescue", StdErr: stderr})
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

				Expect(service.actionEnsureCorrectBoot("rescue", nil)).Should(BeAssignableToTypeOf(actionComplete{}))
				Expect(host.Spec.Status.ErrorType).To(Equal(infrav1.ErrorType("")))
			},
			Entry("without errorType", "", infrav1.ErrorType("")),
			Entry("with errorType", "", infrav1.ErrorTypeHardwareRebootFailed),
		)

		DescribeTable("hostName = rescue, varying error type and ssh client response - robot client giving all positive results, no timeouts",
			func(
				stdout, stderr string,
				err error,
				hostErrorType infrav1.ErrorType,
				expectedActionResult actionResult,
				expectedHostErrorType infrav1.ErrorType,
			) {
				sshMock := &sshmock.Client{}

				sshMock.On("GetHostName").Return(sshclient.Output{StdOut: stdout, StdErr: stderr, Err: err})
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

				Expect(service.actionEnsureCorrectBoot("rescue", nil)).Should(BeAssignableToTypeOf(expectedActionResult))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
			},
			Entry("hostName == rescue, error is set", "rescue", "", errors.New("testerror"), infrav1.ErrorType(""), actionError{}, infrav1.ErrorType("")),
			Entry("timeout, no errorType", "", "", timeout, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHRebootTooSlow),
			Entry("stderr set", "", "std error", nil, infrav1.ErrorType(""), actionError{}, infrav1.ErrorType("")),
			Entry("stderr and errorType set", "", "std error", nil, infrav1.ErrorTypeSoftwareRebootTooSlow, actionError{}, infrav1.ErrorTypeSoftwareRebootTooSlow),
			Entry("timeout,ErrorType == ErrorTypeSoftwareRebootTooSlow", "", "", timeout, infrav1.ErrorTypeSoftwareRebootTooSlow, actionContinue{}, infrav1.ErrorTypeSoftwareRebootTooSlow),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootTooSlow", "", "", timeout, infrav1.ErrorTypeHardwareRebootTooSlow, actionContinue{}, infrav1.ErrorTypeHardwareRebootTooSlow),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootFailed", "", "", timeout, infrav1.ErrorTypeHardwareRebootFailed, actionContinue{}, infrav1.ErrorTypeHardwareRebootFailed),
			Entry("timeout,ErrorType == ErrorTypeSoftwareRebootNotStarted", "", "", timeout, infrav1.ErrorTypeSoftwareRebootNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareRebootTooSlow),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootNotStarted", "", "", timeout, infrav1.ErrorTypeHardwareRebootNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareRebootTooSlow),
			Entry("hostname != rescue", "fedoramachine", "", nil, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSoftwareRebootNotStarted),
			Entry("hostname != rescue, ErrorType == ErrorTypeSoftwareRebootNotStarted", "fedoramachine", "", nil, infrav1.ErrorTypeSoftwareRebootNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareRebootNotStarted),
			Entry("hostname != rescue, ErrorType == ErrorTypeHardwareRebootNotStarted", "fedoramachine", "", nil, infrav1.ErrorTypeHardwareRebootNotStarted, actionContinue{}, infrav1.ErrorTypeHardwareRebootNotStarted),
		)

		// Test with different reset type only software on machine
		DescribeTable("Different reset types",
			func(
				stdout string,
				err error,
				rebootTypes []infrav1.RebootType,
				hostErrorType infrav1.ErrorType,
				expectedHostErrorType infrav1.ErrorType,
				expectedRebootType infrav1.RebootType,
			) {
				sshMock := &sshmock.Client{}
				sshMock.On("GetHostName").Return(sshclient.Output{StdOut: stdout, StdErr: "", Err: err})
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					// Make sure that timeouts are exceeded to trigger escalation step
					helpers.WithError(hostErrorType, "", 1, metav1.NewTime(time.Now().Add(-time.Hour))),
					helpers.WithRebootTypes(rebootTypes),
				)
				service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

				Expect(service.actionEnsureCorrectBoot("rescue", nil)).Should(BeAssignableToTypeOf(actionContinue{}))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectedRebootType != infrav1.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", bareMetalHostID, expectedRebootType)).To(BeTrue())
				}

			},
			Entry("timeout, no errorType, only hw reset", "", timeout, []infrav1.RebootType{infrav1.RebootTypeHardware}, infrav1.ErrorTypeSSHRebootTooSlow, infrav1.ErrorTypeHardwareRebootTooSlow, infrav1.RebootTypeHardware),
			Entry("hostname != rescue, only hw reset", "fedoramachine", nil, []infrav1.RebootType{infrav1.RebootTypeHardware}, infrav1.ErrorType(""), infrav1.ErrorTypeHardwareRebootNotStarted, infrav1.RebootType("")),
			Entry("hostname != rescue", "", timeout, []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, infrav1.ErrorTypeSSHRebootTooSlow, infrav1.ErrorTypeSoftwareRebootTooSlow, infrav1.RebootTypeSoftware),
			Entry("hostname != rescue, only hw reset, errorType =ErrorTypeSSHRebootNotStarted", "fedoramachine", nil, []infrav1.RebootType{infrav1.RebootTypeHardware}, infrav1.ErrorTypeSSHRebootNotStarted, infrav1.ErrorTypeHardwareRebootNotStarted, infrav1.RebootTypeHardware),
			Entry("hostname != rescue", "fedoramachine", nil, []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, infrav1.ErrorTypeSSHRebootNotStarted, infrav1.ErrorTypeSoftwareRebootNotStarted, infrav1.RebootTypeSoftware),
			Entry("hostname != rescue, errorType = ErrorTypeSoftwareRebootNotStarted", "fedoramachine", nil, []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, infrav1.ErrorTypeSoftwareRebootNotStarted, infrav1.ErrorTypeHardwareRebootNotStarted, infrav1.RebootTypeHardware),
			Entry("hostname != rescue, errorType = ErrorTypeHardwareRebootNotStarted", "fedoramachine", nil, []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, infrav1.ErrorTypeHardwareRebootNotStarted, infrav1.ErrorTypeHardwareRebootNotStarted, infrav1.RebootTypeHardware),
		)

		// Test with reached timeouts
		DescribeTable("Different timeouts",
			func(
				hostErrorType infrav1.ErrorType,
				lastUpdated time.Time,
				expectedHostErrorType infrav1.ErrorType,
				expectedRebootType infrav1.RebootType,
			) {
				sshMock := &sshmock.Client{}
				sshMock.On("GetHostName").Return(sshclient.Output{Err: timeout})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(hostErrorType, "", 1, metav1.Time{Time: lastUpdated}),
				)
				service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

				Expect(service.actionEnsureCorrectBoot("rescue", nil)).Should(BeAssignableToTypeOf(actionContinue{}))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectedRebootType != infrav1.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", bareMetalHostID, expectedRebootType)).To(BeTrue())
				}

			},
			Entry("timed out hw reset", infrav1.ErrorTypeHardwareRebootTooSlow, time.Now().Add(-time.Hour), infrav1.ErrorTypeHardwareRebootFailed, infrav1.RebootTypeHardware),
			Entry("timed out failed hw reset", infrav1.ErrorTypeHardwareRebootFailed, time.Now().Add(-time.Hour), infrav1.ErrorTypeHardwareRebootFailed, infrav1.RebootTypeHardware),
			Entry("timed out sw reset", infrav1.ErrorTypeSoftwareRebootTooSlow, time.Now().Add(-5*time.Minute), infrav1.ErrorTypeHardwareRebootTooSlow, infrav1.RebootTypeHardware),
			Entry("not timed out hw reset", infrav1.ErrorTypeHardwareRebootTooSlow, time.Now().Add(-30*time.Minute), infrav1.ErrorTypeHardwareRebootTooSlow, infrav1.RebootType("")),
			Entry("not timed out failed hw reset", infrav1.ErrorTypeHardwareRebootFailed, time.Now().Add(-30*time.Minute), infrav1.ErrorTypeHardwareRebootFailed, infrav1.RebootType("")),
			Entry("not timed out sw reset", infrav1.ErrorTypeSoftwareRebootTooSlow, time.Now().Add(-3*time.Minute), infrav1.ErrorTypeSoftwareRebootTooSlow, infrav1.RebootType("")),
		)
	})

	Context("hostname rescue vs machinename", func() {
		osSSHPort := 23
		DescribeTable("vary hostname and see whether rescue gets triggered",
			func(
				stdout, stderr string,
				err error,
				hostName string,
				osSSHPort *int,
				hostErrorType infrav1.ErrorType,
				expectedActionResult actionResult,
				expectedHostErrorType infrav1.ErrorType,
				expectsRescueCall bool,
			) {
				sshMock := &sshmock.Client{}
				sshMock.On("GetHostName").Return(sshclient.Output{StdOut: stdout, StdErr: stderr, Err: err})
				sshMock.On("Reboot").Return(sshclient.Output{})

				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", bareMetalHostID, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", bareMetalHostID, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

				Expect(service.actionEnsureCorrectBoot(hostName, osSSHPort)).Should(BeAssignableToTypeOf(expectedActionResult))
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectsRescueCall {
					Expect(robotMock.AssertCalled(GinkgoT(), "GetBootRescue", bareMetalHostID)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "GetBootRescue", bareMetalHostID)).To(BeTrue())
				}
			},
			Entry("hostname == rescue", "fedoramachine", "", nil, "rescue", nil, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSoftwareRebootNotStarted, true),
			Entry("hostname == rescue", "fedoramachine", "", nil, "rescue", &osSSHPort, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHRebootNotStarted, true),
			Entry("hostname == machinename", "rescue", "", nil, "machinename", &osSSHPort, infrav1.ErrorType(""), actionContinue{}, infrav1.ErrorTypeSSHRebootNotStarted, false),
			Entry("hostname == machinename, stdout = othermachine", "othermachine", "", nil, "machinename", &osSSHPort, infrav1.ErrorType(""), actionError{}, infrav1.ErrorType(""), false),
			Entry("ErrType == ErrorTypeSSHRebootNotStarted, hostName = rescue, stdout != rescue", "fedoramachine", "", nil, "rescue", &osSSHPort, infrav1.ErrorTypeSSHRebootNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareRebootNotStarted, true),
			Entry("ErrType == ErrorTypeSSHRebootNotStarted, hostName != rescue, stdout == rescue", "rescue", "", nil, "machinename", &osSSHPort, infrav1.ErrorTypeSSHRebootNotStarted, actionContinue{}, infrav1.ErrorTypeSoftwareRebootNotStarted, false),
		)
	})
})

var _ = Describe("ensureSSHKey", func() {
	defaultFingerPrint := "my-fingerprint"
	DescribeTable("ensureSSHKey",
		func(hetznerSSHKeys []models.Key,
			sshSecretKeyRef infrav1.SSHSecretKeyRef,
			expectedFingerprint string,
			expectedActionResult actionResult,
			expectSetSSHKey bool,
		) {
			secret := helpers.GetDefaultSSHSecret("ssh-secret", "default")
			robotMock := robotmock.Client{}
			robotMock.On("SetSSHKey", string(secret.Data[sshSecretKeyRef.Name]), mock.Anything).Return(
				&models.Key{Name: sshSecretKeyRef.Name, Fingerprint: defaultFingerPrint}, nil,
			)
			robotMock.On("ListSSHKeys").Return(hetznerSSHKeys, nil)

			host := helpers.BareMetalHost("test-host", "default")

			service := newTestService(host, &robotMock, nil, nil, nil)

			sshKey, actResult := service.ensureSSHKey(infrav1.SSHSecretRef{
				Name: "secret-name",
				Key:  sshSecretKeyRef,
			}, secret)

			Expect(sshKey.Fingerprint).To(Equal(expectedFingerprint))
			Expect(actResult).Should(BeAssignableToTypeOf(expectedActionResult))
			if expectSetSSHKey {
				Expect(robotMock.AssertCalled(GinkgoT(), "SetSSHKey", string(secret.Data[sshSecretKeyRef.Name]), mock.Anything)).To(BeTrue())
			} else {
				Expect(robotMock.AssertNotCalled(GinkgoT(), "SetSSHKey", string(secret.Data[sshSecretKeyRef.Name]), mock.Anything)).To(BeTrue())
			}
		},
		Entry(
			"empty list",
			nil,
			infrav1.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			defaultFingerPrint,
			actionComplete{},
			true,
		),
		Entry("secret in list",
			[]models.Key{
				{
					Name:        "my-name",
					Fingerprint: "my-fingerprint",
				},
			},
			infrav1.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			"my-fingerprint",
			actionComplete{},
			false,
		),
		Entry(
			"secret not in list",
			[]models.Key{
				{
					Name:        "secret2",
					Fingerprint: "my fingerprint",
				},
				{
					Name:        "secret3",
					Fingerprint: "my fingerprint",
				},
			},
			infrav1.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			defaultFingerPrint,
			actionComplete{},
			true,
		),
	)
})

var _ = Describe("checkHostNameInput", func() {
	DescribeTable("checkHostNameInput",
		func(out sshclient.Output,
			hostName string,
			osSSHPort *int,
			getHostNameError error,
			expectedIsInCorrectBoot bool,
			expectedIsTimeout bool,
			expectedIsConnectionRefused bool,
			expectedErrMessage *string,
			expectGetHostNameCall bool,
		) {

			sshMock := sshmock.Client{}
			sshMock.On("GetHostName").Return(out)

			secondarySSHMock := sshmock.Client{}
			secondarySSHMock.On("GetHostName").Return(sshclient.Output{Err: getHostNameError})

			isInCorrectBoot, isTimeout, isConnectionRefused, err := checkHostNameOutput(out, &secondarySSHMock, hostName, osSSHPort)
			Expect(isInCorrectBoot).To(Equal(expectedIsInCorrectBoot))
			Expect(isTimeout).To(Equal(expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(expectedIsConnectionRefused))
			if expectedErrMessage != nil {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(*expectedErrMessage))
			}
			if expectGetHostNameCall {
				Expect(secondarySSHMock.AssertCalled(GinkgoT(), "GetHostName")).To(BeTrue())
			} else {
				Expect(secondarySSHMock.AssertNotCalled(GinkgoT(), "GetHostName")).To(BeTrue())
			}
		},
		Entry(
			"correct boot - rescue",
			sshclient.Output{StdOut: "rescue"}, // out sshclient.Output
			rescue,                             // hostName string
			nil,                                // osSSHPort *int
			nil,                                // getHostNameError error
			true,                               // expectedIsInCorrectBoot bool
			false,                              // expectedIsTimeout bool
			false,                              // expectedIsConnectionRefused bool
			nil,                                // expectedErrMessage *string
			false,                              // expectGetHostNameCall bool
		),
		Entry(
			"correct boot - os",
			sshclient.Output{StdOut: "os"}, // out sshclient.Output
			"os",                           // hostName string
			nil,                            // osSSHPort *int
			nil,                            // getHostNameError error
			true,                           // expectedIsInCorrectBoot bool
			false,                          // expectedIsTimeout bool
			false,                          // expectedIsConnectionRefused bool
			nil,                            // expectedErrMessage *string
			false,                          // expectGetHostNameCall bool
		),
		Entry(
			"incorrect boot - os",
			sshclient.Output{StdOut: "os"},        // out sshclient.Output
			"os-other",                            // hostName string
			nil,                                   // osSSHPort *int
			nil,                                   // getHostNameError error
			false,                                 // expectedIsInCorrectBoot bool
			false,                                 // expectedIsTimeout bool
			false,                                 // expectedIsConnectionRefused bool
			pointer.String("unexpected hostname"), // expectedErrMessage *string
			false,                                 // expectGetHostNameCall bool
		),
		Entry(
			"timeout error",
			sshclient.Output{Err: timeout}, // out sshclient.Output
			"os-other",                     // hostName string
			nil,                            // osSSHPort *int
			nil,                            // getHostNameError error
			false,                          // expectedIsInCorrectBoot bool
			true,                           // expectedIsTimeout bool
			false,                          // expectedIsConnectionRefused bool
			nil,                            // expectedErrMessage *string
			false,                          // expectGetHostNameCall bool
		),
		Entry(
			"stdErr non-empty",
			sshclient.Output{StdErr: "some error"}, // out sshclient.Output
			"os-other",                             // hostName string
			nil,                                    // osSSHPort *int
			nil,                                    // getHostNameError error
			false,                                  // expectedIsInCorrectBoot bool
			false,                                  // expectedIsTimeout bool
			false,                                  // expectedIsConnectionRefused bool
			pointer.String("failed to get host name via ssh. StdErr: some error"), // expectedErrMessage *string
			false, // expectGetHostNameCall bool
		),
		Entry(
			"incorrect boot - os",
			sshclient.Output{StdOut: ""},     // out sshclient.Output
			"os-other",                       // hostName string
			nil,                              // osSSHPort *int
			nil,                              // getHostNameError error
			false,                            // expectedIsInCorrectBoot bool
			false,                            // expectedIsTimeout bool
			false,                            // expectedIsConnectionRefused bool
			pointer.String("empty hostname"), // expectedErrMessage *string
			false,                            // expectGetHostNameCall bool
		),
		Entry(
			"unable to authenticate - osPort != 22",
			sshclient.Output{Err: errors.New("ssh error: ssh: unable to authenticate")}, // out sshclient.Output
			"os-other",                      // hostName string
			pointer.Int(21),                 // osSSHPort *int
			nil,                             // getHostNameError error
			false,                           // expectedIsInCorrectBoot bool
			false,                           // expectedIsTimeout bool
			false,                           // expectedIsConnectionRefused bool
			pointer.String("wrong ssh key"), // expectedErrMessage *string
			false,                           // expectGetHostNameCall bool
		),
		Entry(
			"unable to authenticate - osPort == 22, no error for getHostName",
			sshclient.Output{Err: errors.New("ssh error: ssh: unable to authenticate")}, // out sshclient.Output
			"os-other",      // hostName string
			pointer.Int(22), // osSSHPort *int
			nil,             // getHostNameError error
			false,           // expectedIsInCorrectBoot bool
			false,           // expectedIsTimeout bool
			false,           // expectedIsConnectionRefused bool
			nil,             // expectedErrMessage *string
			true,            // expectGetHostNameCall bool
		),
		Entry(
			"unable to authenticate - osPort == 22, error for getHostName",
			sshclient.Output{Err: errors.New("ssh error: ssh: unable to authenticate")}, // out sshclient.Output
			"os-other",                      // hostName string
			pointer.Int(22),                 // osSSHPort *int
			errors.New("non-nil error"),     // getHostNameError error
			false,                           // expectedIsInCorrectBoot bool
			false,                           // expectedIsTimeout bool
			false,                           // expectedIsConnectionRefused bool
			pointer.String("wrong ssh key"), // expectedErrMessage *string
			true,                            // expectGetHostNameCall bool
		),
		Entry(
			"connection refused - osPort == 22",
			sshclient.Output{Err: errors.New("ssh error: connect: connection refused")}, // out sshclient.Output
			"os-other",      // hostName string
			pointer.Int(22), // osSSHPort *int
			nil,             // getHostNameError error
			false,           // expectedIsInCorrectBoot bool
			false,           // expectedIsTimeout bool
			true,            // expectedIsConnectionRefused bool
			nil,             // expectedErrMessage *string
			false,           // expectGetHostNameCall bool
		),
		Entry(
			"connection refused - osPort != 22, no error for getHostName",
			sshclient.Output{Err: errors.New("ssh error: connect: connection refused")}, // out sshclient.Output
			"os-other",      // hostName string
			pointer.Int(21), // osSSHPort *int
			nil,             // getHostNameError error
			false,           // expectedIsInCorrectBoot bool
			false,           // expectedIsTimeout bool
			false,           // expectedIsConnectionRefused bool
			nil,             // expectedErrMessage *string
			true,            // expectGetHostNameCall bool
		),
		Entry(
			"connection refused - osPort != 22, error for getHostName",
			sshclient.Output{Err: errors.New("ssh error: connect: connection refused")}, // out sshclient.Output
			"os-other",                  // hostName string
			pointer.Int(21),             // osSSHPort *int
			errors.New("non-nil error"), // getHostNameError error
			false,                       // expectedIsInCorrectBoot bool
			false,                       // expectedIsTimeout bool
			true,                        // expectedIsConnectionRefused bool
			nil,                         // expectedErrMessage *string
			true,                        // expectGetHostNameCall bool
		),
	)
})

var _ = Describe("actionRegistering", func() {
	DescribeTable("actionRegistering",
		func(
			storageStdOut string,
			includeRootDeviceHints bool,
			expectedActionResult actionResult,
			expectedErrorMessage *string,
		) {

			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsRAM").Return(sshclient.Output{StdOut: "10000"})
			sshMock.On("GetHardwareDetailsStorage").Return(sshclient.Output{
				StdOut: storageStdOut,
			})
			sshMock.On("GetHardwareDetailsNics").Return(sshclient.Output{
				StdOut: `name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ipv4="23.88.6.239/26" speedMbps="1000"
		name="eth0" model="Realtek Semiconductor Co., Ltd. RTL8111/8168/8411 PCI Express Gigabit Ethernet Controller (rev 15)" mac="a8:a1:59:94:19:42" ipv6="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
			})
			sshMock.On("GetHardwareDetailsCPUArch").Return(sshclient.Output{StdOut: "myarch"})
			sshMock.On("GetHardwareDetailsCPUModel").Return(sshclient.Output{StdOut: "mymodel"})
			sshMock.On("GetHardwareDetailsCPUClockGigahertz").Return(sshclient.Output{StdOut: "42654"})
			sshMock.On("GetHardwareDetailsCPUFlags").Return(sshclient.Output{StdOut: "flag1 flag2 flag3"})
			sshMock.On("GetHardwareDetailsCPUThreads").Return(sshclient.Output{StdOut: "123"})
			sshMock.On("GetHardwareDetailsCPUCores").Return(sshclient.Output{StdOut: "12"})

			var host *infrav1.HetznerBareMetalHost
			if includeRootDeviceHints {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithRootDeviceHints(),
					helpers.WithIP(),
				)
			} else {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithIP(),
				)
			}

			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionRegistering()
			Expect(actResult).Should(BeAssignableToTypeOf(expectedActionResult))
			Expect(host.Spec.Status.HardwareDetails).ToNot(BeNil())
			if expectedErrorMessage != nil {
				Expect(host.Spec.Status.ErrorMessage).To(Equal(*expectedErrorMessage))
			}
		},
		Entry(
			"working example",
			`NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
		NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
		NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			true,
			actionComplete{},
			nil,
		),
		Entry(
			"wwn does not fit to storage devices",
			`NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
			NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
			NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			true,
			actionFailed{},
			pointer.String("no storage device found with root device hints"),
		),
		Entry(
			"no root device hints",
			`NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
			NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
			NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			false,
			actionFailed{},
			pointer.String(infrav1.ErrorMessageMissingRootDeviceHints),
		),
	)
})

var _ = Describe("getImageDetails", func() {
	DescribeTable("getImageDetails",
		func(image infrav1.Image,
			expectedImagePath string,
			expectedNeedsDownload bool,
			expectedErrorMessage string,
		) {
			imagePath, needsDownload, errorMessage := getImageDetails(image)
			Expect(imagePath).To(Equal(expectedImagePath))
			Expect(needsDownload).To(Equal(expectedNeedsDownload))
			Expect(errorMessage).To(Equal(expectedErrorMessage))
		},
		Entry(
			"name and url specified, tar.gz suffix",
			infrav1.Image{
				Name: "imageName",
				URL:  "https://mytargz.tar.gz",
				Path: "",
			}, // image infrav1.Image
			"/root/imageName.tar.gz", // expectedImagePath string
			true,                     // expectedNeedsDownload bool
			"",                       // expectedErrorMessage string
		),
		Entry(
			"name and url specified, tgz suffix",
			infrav1.Image{
				Name: "imageName",
				URL:  "https://mytargz.tgz",
				Path: "",
			}, // image infrav1.Image
			"/root/imageName.tgz", // expectedImagePath string
			true,                  // expectedNeedsDownload bool
			"",                    // expectedErrorMessage string
		),
		Entry(
			"name and url specified, wrong suffix",
			infrav1.Image{
				Name: "imageName",
				URL:  "https://mytargz.tgx",
				Path: "",
			}, // image infrav1.Image
			"",                       // expectedImagePath string
			false,                    // expectedNeedsDownload bool
			"wrong image url suffix", // expectedErrorMessage string
		),
		Entry(
			"path specified",
			infrav1.Image{
				Name: "",
				URL:  "",
				Path: "imagePath",
			}, // image infrav1.Image
			"imagePath", // expectedImagePath string
			false,       // expectedNeedsDownload bool
			"",          // expectedErrorMessage string
		),
		Entry(
			"neither specified",
			infrav1.Image{
				Name: "imageName",
				URL:  "",
				Path: "",
			}, // image infrav1.Image
			"",    // expectedImagePath string
			false, // expectedNeedsDownload bool
			"invalid image - need to specify either name and url or path", // expectedErrorMessage string
		),
	)
})

var _ = Describe("actionEnsureProvisioned", func() {
	DescribeTable("actionEnsureProvisioned",
		func(
			cloudInitStatus string,
			expectedActionResult actionResult,
			expectedErrorType infrav1.ErrorType,
			shouldCallReboot bool,
		) {

			sshMock := &sshmock.Client{}
			sshMock.On("CloudInitStatus").Return(sshclient.Output{StdOut: cloudInitStatus})
			sshMock.On("Reboot").Return(sshclient.Output{})

			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHSpecInclPorts(),
				helpers.WithIP(),
				helpers.WithConsumerRef(),
			)

			robotMock := robotmock.Client{}
			robotMock.On("SetBMServerName", bareMetalHostID, infrav1.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)

			actResult := service.actionEnsureProvisioned()
			Expect(actResult).Should(BeAssignableToTypeOf(expectedActionResult))
			if expectedErrorType != infrav1.ErrorType("") {
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedErrorType))
			}
			if shouldCallReboot {
				Expect(sshMock.AssertCalled(GinkgoT(), "Reboot")).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "Reboot")).To(BeTrue())
			}
		},
		Entry(
			"status running",
			"status: running",     // cloudInitStatus string
			actionContinue{},      // expectedActionResult actionResult
			infrav1.ErrorType(""), // expectedErrorType string
			false,                 // shouldCallReboot bool
		),
		Entry(
			"status done",
			"status: done",        // cloudInitStatus string
			actionComplete{},      // expectedActionResult actionResult
			infrav1.ErrorType(""), // expectedErrorType string
			false,                 // shouldCallReboot bool
		),
		Entry(
			"status disabled",
			"status: disabled",                   // cloudInitStatus string
			actionContinue{},                     // expectedActionResult actionResult
			infrav1.ErrorTypeSSHRebootNotStarted, // expectedErrorType string
			true,                                 // shouldCallReboot bool
		),
	)
})

var _ = Describe("actionProvisioned", func() {
	DescribeTable("actionProvisioned",
		func(
			shouldHaveRebootAnnotation bool,
			rebooted bool,
			rebootFinished bool,
			expectedActionResult actionResult,
			expectRebootAnnotation bool,
			expectRebootInStatus bool,
		) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHSpecInclPorts(),
				helpers.WithIP(),
				helpers.WithConsumerRef(),
			)

			if shouldHaveRebootAnnotation {
				host.SetAnnotations(map[string]string{infrav1.RebootAnnotation: "reboot"})
			}

			host.Status.Rebooted = rebooted

			sshMock := &sshmock.Client{}
			var hostNameOutput sshclient.Output
			if rebootFinished {
				hostNameOutput = sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + host.Spec.ConsumerRef.Name}
			} else {
				hostNameOutput = sshclient.Output{Err: timeout}
			}
			sshMock.On("GetHostName").Return(hostNameOutput)
			sshMock.On("Reboot").Return(sshclient.Output{})

			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionProvisioned()
			Expect(actResult).Should(BeAssignableToTypeOf(expectedActionResult))
			Expect(host.Status.Rebooted).To(Equal(expectRebootInStatus))
			Expect(hasRebootAnnotation(*host)).To(Equal(expectRebootAnnotation))

			if shouldHaveRebootAnnotation && !rebooted {
				Expect(sshMock.AssertCalled(GinkgoT(), "Reboot")).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "Reboot")).To(BeTrue())
			}
		},
		Entry(
			"reboot desired, but not performed yet",
			true,             // shouldHaveRebootAnnotation bool
			false,            // rebooted bool
			false,            // rebootFinished bool
			actionContinue{}, // expectedActionResult actionResult
			true,             // expectRebootAnnotation bool
			true,             // expectRebootInStatus bool,
		),
		Entry(
			"reboot desired, and already performed, not finished",
			true,             // shouldHaveRebootAnnotation bool
			true,             // rebooted bool
			false,            // rebootFinished bool
			actionContinue{}, // expectedActionResult actionResult
			true,             // expectRebootAnnotation bool
			true,             // expectRebootInStatus bool,
		),
		Entry(
			"reboot desired, and already performed, finished",
			true,             // shouldHaveRebootAnnotation bool
			true,             // rebooted bool
			true,             // rebootFinished bool
			actionComplete{}, // expectedActionResult actionResult
			false,            // expectRebootAnnotation bool
			false,            // expectRebootInStatus bool,
		),
		Entry(
			"no reboot desired",
			false,            // shouldHaveRebootAnnotation bool
			false,            // rebooted bool
			false,            // rebootFinished bool
			actionComplete{}, // expectedActionResult actionResult
			false,            // expectRebootAnnotation bool
			false,            // expectRebootInStatus bool,
		),
	)
})
