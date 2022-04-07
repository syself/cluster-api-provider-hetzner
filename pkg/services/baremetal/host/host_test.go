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

var _ = Describe("handleIncompleteBootError", func() {
	Context("correct hostname == rescue", func() {
		DescribeTable("hostName = rescue, varying error type and ssh client response - robot client giving all positive results, no timeouts",
			func(
				isRebootIntoRescue bool,
				isTimeOut bool,
				isConnectionRefused bool,
				hostErrorType infrav1.ErrorType,
				expectedReturnError error,
				expectedHostErrorType infrav1.ErrorType,
			) {
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
				service := newTestService(host, &robotMock, nil, nil, nil)

				if expectedReturnError == nil {
					Expect(service.handleIncompleteBootError(isRebootIntoRescue, isTimeOut, isConnectionRefused)).To(Succeed())
				} else {
					Expect(service.handleIncompleteBootError(isRebootIntoRescue, isTimeOut, isConnectionRefused)).Should(Equal(expectedReturnError))
				}

				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
			},
			Entry("timeout, no errorType",
				true,                              // isRebootIntoRescue bool
				true,                              // isTimeOut bool
				false,                             // isConnectionRefused bool
				infrav1.ErrorType(""),             // hostErrorType infrav1.ErrorType
				nil,                               //	expectedReturnError error
				infrav1.ErrorTypeSSHRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("timeout,ErrorType == ErrorTypeSoftwareRebootTooSlow",
				true,                                   // isRebootIntoRescue bool
				true,                                   // isTimeOut bool
				false,                                  // isConnectionRefused bool
				infrav1.ErrorTypeSoftwareRebootTooSlow, // hostErrorType infrav1.ErrorType
				nil,                                    //	expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootTooSlow",
				true,                                   // isRebootIntoRescue bool
				true,                                   // isTimeOut bool
				false,                                  // isConnectionRefused bool
				infrav1.ErrorTypeHardwareRebootTooSlow, // hostErrorType infrav1.ErrorType
				nil,                                    //	expectedReturnError error
				infrav1.ErrorTypeHardwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootFailed",
				true,                                  // isRebootIntoRescue bool
				true,                                  // isTimeOut bool
				false,                                 // isConnectionRefused bool
				infrav1.ErrorTypeHardwareRebootFailed, // hostErrorType infrav1.ErrorType
				nil,                                   //	expectedReturnError error
				infrav1.ErrorTypeHardwareRebootFailed, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("timeout,ErrorType == ErrorTypeSoftwareRebootNotStarted",
				true,  // isRebootIntoRescue bool
				true,  // isTimeOut bool
				false, // isConnectionRefused bool
				infrav1.ErrorTypeSoftwareRebootNotStarted, // hostErrorType infrav1.ErrorType
				nil,                                    //	expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootNotStarted",
				true,  // isRebootIntoRescue bool
				true,  // isTimeOut bool
				false, // isConnectionRefused bool
				infrav1.ErrorTypeHardwareRebootNotStarted, // hostErrorType infrav1.ErrorType
				nil,                                    //	expectedReturnError error
				infrav1.ErrorTypeHardwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("timeout,ErrorType == ErrorTypeSSHRebootNotStarted",
				true,                                 // isRebootIntoRescue bool
				false,                                // isTimeOut bool
				false,                                // isConnectionRefused bool
				infrav1.ErrorTypeSSHRebootNotStarted, // hostErrorType infrav1.ErrorType
				nil,                                  //	expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootNotStarted, // expectedHostErrorType infrav1.ErrorType
			),
			Entry("wrong boot",
				false,                 // isRebootIntoRescue bool
				false,                 // isTimeOut bool
				false,                 // isConnectionRefused bool
				infrav1.ErrorType(""), // hostErrorType infrav1.ErrorType
				nil,                   //	expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootNotStarted, // expectedHostErrorType infrav1.ErrorType
			),
		)

		// Test with different reset type only software on machine
		DescribeTable("Different reset types",
			func(
				isTimeOut bool,
				isConnectionRefused bool,
				rebootTypes []infrav1.RebootType,
				hostErrorType infrav1.ErrorType,
				expectedHostErrorType infrav1.ErrorType,
				expectedRebootType infrav1.RebootType,
			) {
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
				service := newTestService(host, &robotMock, nil, nil, nil)

				Expect(service.handleIncompleteBootError(true, isTimeOut, isConnectionRefused)).To(Succeed())
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectedRebootType != infrav1.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", bareMetalHostID, expectedRebootType)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", bareMetalHostID, mock.Anything)).To(BeTrue())
				}
			},
			Entry("timeout, no errorType, only hw reset",
				true,  // isTimeOut bool
				false, // isConnectionRefused bool
				[]infrav1.RebootType{infrav1.RebootTypeHardware}, // rebootTypes []infrav1.RebootType
				infrav1.ErrorTypeSSHRebootTooSlow,                // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootNotStarted,        // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,                       // expectedRebootType infrav1.RebootType
			),
			Entry("wrong boot, only hw reset",
				false, // isTimeOut bool
				false, // isConnectionRefused bool
				[]infrav1.RebootType{infrav1.RebootTypeHardware}, // rebootTypes []infrav1.RebootType
				infrav1.ErrorType(""),                            // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootNotStarted,        // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,                       // expectedRebootType infrav1.RebootType
			),
			Entry("wrong boot, only hw reset, errorType =ErrorTypeSSHRebootNotStarted",
				false, // isTimeOut bool
				false, // isConnectionRefused bool
				[]infrav1.RebootType{infrav1.RebootTypeHardware}, // rebootTypes []infrav1.RebootType
				infrav1.ErrorTypeSSHRebootNotStarted,             // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootNotStarted,        // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,                       // expectedRebootType infrav1.RebootType
			),
			Entry("wrong boot, errorType =ErrorTypeSSHRebootNotStarted",
				false, // isTimeOut bool
				false, // isConnectionRefused bool
				[]infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, // rebootTypes []infrav1.RebootType
				infrav1.ErrorTypeSSHRebootNotStarted,                                         // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeSoftwareRebootNotStarted,                                    // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeSoftware,                                                   // expectedRebootType infrav1.RebootType
			),
			Entry("wrong boot,  errorType =ErrorTypeSoftwareRebootNotStarted",
				false, // isTimeOut bool
				false, // isConnectionRefused bool
				[]infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, // rebootTypes []infrav1.RebootType
				infrav1.ErrorTypeSoftwareRebootNotStarted,                                    // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootNotStarted,                                    // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,                                                   // expectedRebootType infrav1.RebootType
			),
			Entry("wrong boot,  errorType =ErrorTypeHardwareRebootNotStarted",
				false, // isTimeOut bool
				false, // isConnectionRefused bool
				[]infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware}, // rebootTypes []infrav1.RebootType
				infrav1.ErrorTypeHardwareRebootNotStarted,                                    // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootNotStarted,                                    // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,                                                   // expectedRebootType infrav1.RebootType
			),
		)

		// Test with reached timeouts
		DescribeTable("Different timeouts",
			func(
				hostErrorType infrav1.ErrorType,
				lastUpdated time.Time,
				expectedHostErrorType infrav1.ErrorType,
				expectedRebootType infrav1.RebootType,
			) {
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
				service := newTestService(host, &robotMock, nil, nil, nil)

				Expect(service.handleIncompleteBootError(true, true, false)).To(Succeed())
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectedRebootType != infrav1.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", bareMetalHostID, expectedRebootType)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", bareMetalHostID, mock.Anything)).To(BeTrue())
				}
			},
			Entry(
				"timed out hw reset",                   // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootTooSlow, // hostErrorType infrav1.ErrorType
				time.Now().Add(-time.Hour),             // lastUpdated time.Time
				infrav1.ErrorTypeHardwareRebootFailed,  // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,             // expectedRebootType infrav1.RebootType
			),
			Entry(
				"timed out failed hw reset",           // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootFailed, // hostErrorType infrav1.ErrorType
				time.Now().Add(-time.Hour),            // lastUpdated time.Time
				infrav1.ErrorTypeHardwareRebootFailed, // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,            // expectedRebootType infrav1.RebootType
			),
			Entry(
				"timed out sw reset",                   // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeSoftwareRebootTooSlow, // hostErrorType infrav1.ErrorType
				time.Now().Add(-5*time.Minute),         // lastUpdated time.Time
				infrav1.ErrorTypeHardwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootTypeHardware,             // expectedRebootType infrav1.RebootType
			),
			Entry(
				"not timed out hw reset",               // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootTooSlow, // hostErrorType infrav1.ErrorType
				time.Now().Add(-30*time.Minute),        // lastUpdated time.Time
				infrav1.ErrorTypeHardwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootType(""),                 // expectedRebootType infrav1.RebootType
			),
			Entry(
				"not timed out failed hw reset",       // hostErrorType infrav1.ErrorType
				infrav1.ErrorTypeHardwareRebootFailed, // hostErrorType infrav1.ErrorType
				time.Now().Add(-30*time.Minute),       // lastUpdated time.Time
				infrav1.ErrorTypeHardwareRebootFailed, // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootType(""),                // expectedRebootType infrav1.RebootType
			),
			Entry(
				"not timed out sw reset",
				infrav1.ErrorTypeSoftwareRebootTooSlow, // hostErrorType infrav1.ErrorType
				time.Now().Add(-3*time.Minute),         // lastUpdated time.Time
				infrav1.ErrorTypeSoftwareRebootTooSlow, // expectedHostErrorType infrav1.ErrorType
				infrav1.RebootType(""),                 // expectedRebootType infrav1.RebootType
			),
		)
	})

	Context("hostname rescue vs machinename", func() {
		DescribeTable("vary hostname and see whether rescue gets triggered",
			func(
				isRebootIntoRescue bool,
				hostErrorType infrav1.ErrorType,
				expectedReturnError error,
				expectedHostErrorType infrav1.ErrorType,
				expectsRescueCall bool,
			) {
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
				service := newTestService(host, &robotMock, nil, nil, nil)

				if expectedReturnError == nil {
					Expect(service.handleIncompleteBootError(isRebootIntoRescue, false, false)).To(Succeed())
				} else {
					Expect(service.handleIncompleteBootError(isRebootIntoRescue, false, false)).Should(Equal(expectedReturnError))
				}
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedHostErrorType))
				if expectsRescueCall {
					Expect(robotMock.AssertCalled(GinkgoT(), "GetBootRescue", bareMetalHostID)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "GetBootRescue", bareMetalHostID)).To(BeTrue())
				}
			},
			Entry("hostname == rescue",
				true,                  // isRebootIntoRescue bool
				infrav1.ErrorType(""), // hostErrorType infrav1.ErrorType
				nil,                   // expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootNotStarted, // expectedHostErrorType infrav1.ErrorType
				true, // expectsRescueCall bool
			),
			Entry("hostname != rescue",
				false,                 // isRebootIntoRescue bool
				infrav1.ErrorType(""), // hostErrorType infrav1.ErrorType
				nil,                   // expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootNotStarted, // expectedHostErrorType infrav1.ErrorType
				false, // expectsRescueCall bool
			),
			Entry("hostname == rescue, ErrType == ErrorTypeSSHRebootNotStarted",
				true,                                 // isRebootIntoRescue bool
				infrav1.ErrorTypeSSHRebootNotStarted, // hostErrorType infrav1.ErrorType
				nil,                                  // expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootNotStarted, // expectedHostErrorType infrav1.ErrorType
				true, // expectsRescueCall bool
			),
			Entry("hostname != rescue, ErrType == ErrorTypeSSHRebootNotStarted",
				false,                                // isRebootIntoRescue bool
				infrav1.ErrorTypeSSHRebootNotStarted, // hostErrorType infrav1.ErrorType
				nil,                                  // expectedReturnError error
				infrav1.ErrorTypeSoftwareRebootNotStarted, // expectedHostErrorType infrav1.ErrorType
				false, // expectsRescueCall bool
			),
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

var _ = Describe("handleIncompleteBootInstallImage", func() {
	DescribeTable("handleIncompleteBootInstallImage - out.Err",
		func(
			err error,
			rescueActive bool,
			expectedIsTimeout bool,
			expectedIsConnectionRefused bool,
			expectedErrMessage string,
		) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
			)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: rescueActive}, nil)

			service := newTestService(host, &robotMock, nil, nil, nil)

			isTimeout, isConnectionRefused, err := service.handleIncompleteBootRegistering(sshclient.Output{Err: err})
			Expect(isTimeout).To(Equal(expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(expectedIsConnectionRefused))
			if expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry(
			"timeout error",
			timeout, // err error
			true,    // rescueActive bool
			true,    // expectedIsTimeout bool
			false,   // expectedIsConnectionRefused bool
			"",      // expectedErrMessage string
		),
		Entry(
			"authenticationFailed error, rescue active",
			sshclient.ErrAuthenticationFailed, // err error
			true,                              // rescueActive bool
			false,                             // expectedIsTimeout bool
			false,                             // expectedIsConnectionRefused bool
			"",                                // expectedErrMessage string
		),
		Entry(
			"authenticationFailed error, rescue not active",
			sshclient.ErrAuthenticationFailed, // err error
			false,                             // rescueActive bool
			false,                             // expectedIsTimeout bool
			false,                             // expectedIsConnectionRefused bool
			"wrong ssh key",                   // expectedErrMessage string
		),
		Entry(
			"connectionRefused error, rescue active",
			sshclient.ErrConnectionRefused, // err error
			true,                           // rescueActive bool
			false,                          // expectedIsTimeout bool
			false,                          // expectedIsConnectionRefused bool
			"",                             // expectedErrMessage string
		),
		Entry(
			"connectionRefused error, rescue not active",
			sshclient.ErrConnectionRefused, // err error
			false,                          // rescueActive bool
			false,                          // expectedIsTimeout bool
			true,                           // expectedIsConnectionRefused bool
			"",                             // expectedErrMessage string
		),
	)

	DescribeTable("handleIncompleteBootRegistering - toggle stdErr and hostName",
		func(
			hasNilErr bool,
			stdErr string,
			hostName string,
			expectedErrMessage string,
		) {
			var err error
			if !hasNilErr {
				err = errors.New("unknown error")
			}

			out := sshclient.Output{
				StdOut: hostName,
				StdErr: stdErr,
				Err:    err,
			}

			host := helpers.BareMetalHost(
				"test-host",
				"default",
			)

			service := newTestService(host, nil, nil, nil, nil)

			isTimeout, isConnectionRefused, err := service.handleIncompleteBootRegistering(out)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry(
			"stderr not empty",
			true,             // hasNilErr bool
			"command failed", // stdErr string
			"hostName",       // hostName string
			"failed to get host name via ssh. StdErr:", // expectedErrMessage string
		),
		Entry(
			"stderr not empty - err != nil",
			false,            // hasNilErr bool
			"command failed", // stdErr string
			"",               // hostName string
			"unhandled ssh error while getting hostname", // expectedErrMessage string
		),
		Entry(
			"stderr not empty - wrong hostName",
			true,             // hasNilErr bool
			"command failed", // stdErr string
			"",               // hostName string
			"failed to get host name via ssh. StdErr:", // expectedErrMessage string
		),
		Entry(
			"stderr empty - wrong hostName",
			true,                   // hasNilErr bool
			"",                     // stdErr string
			"",                     // hostName string
			"error empty hostname", // expectedErrMessage string
		),
	)
})

var _ = Describe("handleIncompleteBootInstallImage", func() {
	DescribeTable("handleIncompleteBootInstallImage - out.Err",
		func(
			err error,
			getHostNameErrNil bool,
			port int,
			expectedIsTimeout bool,
			expectedIsConnectionRefused bool,
			expectedErrMessage string,
		) {
			sshMock := &sshmock.Client{}
			var getHostNameErr error
			if !getHostNameErrNil {
				getHostNameErr = errors.New("non-nil error")
			}
			sshMock.On("GetHostName").Return(sshclient.Output{Err: getHostNameErr})

			isTimeout, isConnectionRefused, err := handleIncompleteBootInstallImage(sshclient.Output{Err: err}, sshMock, port)
			Expect(isTimeout).To(Equal(expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(expectedIsConnectionRefused))
			if expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry(
			"timeout error",
			timeout, // err error
			true,    // getHostNameErrNil bool
			22,      // port int
			true,    // expectedIsTimeout bool
			false,   // expectedIsConnectionRefused bool
			"",      // expectedErrMessage string
		),
		Entry(
			"authenticationFailed error, port 22, no hostName error",
			sshclient.ErrAuthenticationFailed, // err error
			true,                              // getHostNameErrNil bool
			22,                                // port int
			false,                             // expectedIsTimeout bool
			false,                             // expectedIsConnectionRefused bool
			"",                                // expectedErrMessage string
		),
		Entry(
			"authenticationFailed error, port 22, hostName error",
			sshclient.ErrAuthenticationFailed, // err error
			false,                             // getHostNameErrNil bool
			22,                                // port int
			false,                             // expectedIsTimeout bool
			false,                             // expectedIsConnectionRefused bool
			"wrong ssh key",                   // expectedErrMessage string
		),
		Entry(
			"authenticationFailed error, port != 22",
			sshclient.ErrAuthenticationFailed, // err error
			true,                              // getHostNameErrNil bool
			23,                                // port int
			false,                             // expectedIsTimeout bool
			false,                             // expectedIsConnectionRefused bool
			"wrong ssh key",                   // expectedErrMessage string
		),
		Entry(
			"connectionRefused error, port 22",
			sshclient.ErrConnectionRefused, // err error
			true,                           // getHostNameErrNil bool
			22,                             // port int
			false,                          // expectedIsTimeout bool
			true,                           // expectedIsConnectionRefused bool
			"",                             // expectedErrMessage string
		),
		Entry(
			"connectionRefused error, port != 22, hostname error",
			sshclient.ErrConnectionRefused, // err error
			false,                          // getHostNameErrNil bool
			23,                             // port int
			false,                          // expectedIsTimeout bool
			true,                           // expectedIsConnectionRefused bool
			"",                             // expectedErrMessage string
		),
		Entry(
			"connectionRefused error, port != 22, no hostname error",
			sshclient.ErrConnectionRefused, // err error
			true,                           // getHostNameErrNil bool
			23,                             // port int
			false,                          // expectedIsTimeout bool
			false,                          // expectedIsConnectionRefused bool
			"",                             // expectedErrMessage string
		),
	)

	DescribeTable("handleIncompleteBootInstallImage - StdErr not empty",
		func(
			hasNilErr bool,
			stdErr string,
			hasWrongHostName bool,
			expectedErrMessage string,
		) {
			var err error
			if !hasNilErr {
				err = errors.New("unknown error")
			}
			hostName := "rescue"
			if hasWrongHostName {
				hostName = "wrongHostName"
			}

			out := sshclient.Output{
				StdOut: hostName,
				StdErr: stdErr,
				Err:    err,
			}
			isTimeout, isConnectionRefused, err := handleIncompleteBootInstallImage(out, nil, 22)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry(
			"stderr not empty",
			true,             // hasNilErr bool
			"command failed", // stdErr string
			false,            // hasWrongHostName bool
			"failed to get host name via ssh. StdErr:", // expectedErrMessage string
		),
		Entry(
			"stderr not empty - err != nil",
			false,            // hasNilErr bool
			"command failed", // stdErr string
			false,            // hasWrongHostName bool
			"unhandled ssh error while getting hostname", // expectedErrMessage string
		),
		Entry(
			"stderr not empty - wrong hostName",
			true,             // hasNilErr bool
			"command failed", // stdErr string
			true,             // hasWrongHostName bool
			"failed to get host name via ssh. StdErr:", // expectedErrMessage string
		),
	)

	DescribeTable("handleIncompleteBootInstallImage - wrong hostName",
		func(
			hasNilErr bool,
			stdErr string,
			hostName string,
			expectedErrMessage string,
		) {
			var err error
			if !hasNilErr {
				err = errors.New("unknown error")
			}

			out := sshclient.Output{
				StdOut: hostName,
				StdErr: stdErr,
				Err:    err,
			}
			isTimeout, isConnectionRefused, err := handleIncompleteBootInstallImage(out, nil, 22)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry(
			"empty hostname",
			true,                   // hasNilErr bool
			"",                     // stdErr string
			"",                     // 	hostName string
			"error empty hostname", // expectedErrMessage string
		),
		Entry(
			"empty hostname - err not empty",
			false, // hasNilErr bool
			"",    // stdErr string
			"",    // 	hostName string
			"unhandled ssh error while getting hostname", // expectedErrMessage string
		),
		Entry(
			"empty hostname stderr not empty",
			true,             // hasNilErr bool
			"command failed", // stdErr string
			"",               // 	hostName string
			"failed to get host name via ssh. StdErr:", // expectedErrMessage string
		),
		Entry(
			"hostname == rescue",
			true,     // hasNilErr bool
			"",       // stdErr string
			"rescue", // 	hostName string
			"",       // expectedErrMessage string
		),
		Entry(
			"hostname == otherHostName",
			true,                  // hasNilErr bool
			"",                    // stdErr string
			"otherHostName",       // 	hostName string
			"unexpected hostname", // expectedErrMessage string
		),
	)
})

var _ = Describe("handleIncompleteBootProvisioned", func() {
	DescribeTable("handleIncompleteBootProvisioned",
		func(out sshclient.Output,
			expectedIsTimeout bool,
			expectedIsConnectionRefused bool,
			expectedErrMessage *string,
		) {
			isTimeout, isConnectionRefused, err := handleIncompleteBootProvisioned(out)
			Expect(isTimeout).To(Equal(expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(expectedIsConnectionRefused))
			if expectedErrMessage != nil {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(*expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry(
			"incorrect boot",
			sshclient.Output{StdOut: "wrong_hostname"}, // out sshclient.Output
			false,                                 // expectedIsTimeout bool
			false,                                 // expectedIsConnectionRefused bool
			pointer.String("unexpected hostname"), // expectedErrMessage *string
		),
		Entry(
			"timeout error",
			sshclient.Output{Err: timeout}, // out sshclient.Output
			true,                           // expectedIsTimeout bool
			false,                          // expectedIsConnectionRefused bool
			nil,                            // expectedErrMessage *string
		),
		Entry(
			"stdErr non-empty",
			sshclient.Output{StdErr: "some error"}, // out sshclient.Output
			false,                                  // expectedIsTimeout bool
			false,                                  // expectedIsConnectionRefused bool
			pointer.String("failed to get host name via ssh. StdErr: some error"), // expectedErrMessage *string
		),
		Entry(
			"incorrect boot - empty hostname",
			sshclient.Output{StdOut: ""},     // out sshclient.Output
			false,                            // expectedIsTimeout bool
			false,                            // expectedIsConnectionRefused bool
			pointer.String("empty hostname"), // expectedErrMessage *string
		),
		Entry(
			"unable to authenticate",
			sshclient.Output{Err: sshclient.ErrAuthenticationFailed}, // out sshclient.Output
			false,                           // expectedIsTimeout bool
			false,                           // expectedIsConnectionRefused bool
			pointer.String("wrong ssh key"), // expectedErrMessage *string
		),
		Entry(
			"connection refused",
			sshclient.Output{Err: sshclient.ErrConnectionRefused}, // out sshclient.Output
			false, // expectedIsTimeout bool
			true,  // expectedIsConnectionRefused bool
			nil,   // expectedErrMessage *string
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

			var host *infrav1.HetznerBareMetalHost
			if includeRootDeviceHints {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithRootDeviceHints(),
					helpers.WithIPv4(),
					helpers.WithConsumerRef(),
				)
			} else {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithIPv4(),
					helpers.WithConsumerRef(),
				)
			}
			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName").Return(sshclient.Output{StdOut: "rescue"})
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

	DescribeTable("actionRegistering - incomplete reboot",
		func(
			getHostNameOutput sshclient.Output,
			expectedErrorType infrav1.ErrorType,
		) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithRebootTypes([]infrav1.RebootType{infrav1.RebootTypeHardware}),
				helpers.WithRootDeviceHints(),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName").Return(getHostNameOutput)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", bareMetalHostID).Return(&models.Rescue{Active: false}, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionRegistering()
			Expect(actResult).Should(BeAssignableToTypeOf(actionContinue{}))
			if expectedErrorType != infrav1.ErrorType("") {
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedErrorType))
			}
		},
		Entry(
			"timeout",
			sshclient.Output{Err: timeout},    // getHostNameOutput sshclient.Output
			infrav1.ErrorTypeSSHRebootTooSlow, // expectedErrorType string
		),
		Entry(
			"connectionRefused",
			sshclient.Output{Err: sshclient.ErrConnectionRefused}, // getHostNameOutput sshclient.Output
			infrav1.ErrorTypeConnectionError,                      // expectedErrorType string
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
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHSpecInclPorts(),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName").Return(sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + host.Spec.ConsumerRef.Name})
			sshMock.On("Reboot").Return(sshclient.Output{})
			sshMock.On("CloudInitStatus").Return(sshclient.Output{StdOut: cloudInitStatus})
			sshMock.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})

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

	DescribeTable("actionEnsureProvisioned - incomplete reboot",
		func(
			getHostNameOutput sshclient.Output,
			expectedErrorType infrav1.ErrorType,
		) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHSpecInclPorts(),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName").Return(getHostNameOutput)
			sshMock.On("Reboot").Return(sshclient.Output{})
			sshMock.On("CloudInitStatus").Return(sshclient.Output{StdOut: "status: done"})
			sshMock.On("CheckCloudInitLogsForSigTerm").Return(sshclient.Output{})

			robotMock := robotmock.Client{}
			robotMock.On("SetBMServerName", bareMetalHostID, infrav1.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)

			actResult := service.actionEnsureProvisioned()
			Expect(actResult).Should(BeAssignableToTypeOf(actionContinue{}))
			if expectedErrorType != infrav1.ErrorType("") {
				Expect(host.Spec.Status.ErrorType).To(Equal(expectedErrorType))
			}
		},
		Entry(
			"timeout",
			sshclient.Output{Err: timeout},    // getHostNameOutput sshclient.Output
			infrav1.ErrorTypeSSHRebootTooSlow, // expectedErrorType string
		),
		Entry(
			"connectionRefused",
			sshclient.Output{Err: sshclient.ErrConnectionRefused}, // getHostNameOutput sshclient.Output
			infrav1.ErrorTypeConnectionError,                      // expectedErrorType string
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
				helpers.WithIPv4(),
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
