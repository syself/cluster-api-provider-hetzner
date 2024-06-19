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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/syself/hrobot-go/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	bmmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks"
	robotmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/robot"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

var errTest = fmt.Errorf("test error")

var _ = Describe("SetErrorMessage", func() {
	type testCaseSetErrorMessage struct {
		errorType            infrav1.ErrorType
		errorMessage         string
		hasErrorInStatus     bool
		expectedErrorCount   int
		expectedErrorType    infrav1.ErrorType
		expectedErrorMessage string
	}

	DescribeTable("SetErrorMessage",
		func(tc testCaseSetErrorMessage) {
			var host *infrav1.HetznerBareMetalHost
			if tc.hasErrorInStatus {
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

			host.SetError(tc.errorType, tc.errorMessage)
			Expect(host.Spec.Status.ErrorCount).To(Equal(tc.expectedErrorCount))
			Expect(host.Spec.Status.ErrorMessage).To(Equal(tc.expectedErrorMessage))
			Expect(host.Spec.Status.ErrorType).To(Equal(tc.expectedErrorType))
		},
		Entry("new error with existing one", testCaseSetErrorMessage{
			errorType:            infrav1.RegistrationError, // errorType infrav1.ErrorType
			errorMessage:         "new message",             // errorMessage string
			hasErrorInStatus:     true,                      // hasErrorInStatus bool
			expectedErrorCount:   1,                         // expectedErrorCount int
			expectedErrorType:    infrav1.RegistrationError, //	expectedErrorType
			expectedErrorMessage: "new message",             // expectedErrorMessage
		}),
		Entry("old error with existing one", testCaseSetErrorMessage{
			errorType:            infrav1.PreparationError, // errorType infrav1.ErrorType
			errorMessage:         "first message",          // errorMessage string
			hasErrorInStatus:     true,                     // hasErrorInStatus bool
			expectedErrorCount:   3,                        // expectedErrorCount int
			expectedErrorType:    infrav1.PreparationError, // expectedErrorType
			expectedErrorMessage: "first message",          // expectedErrorMessage
		}),
		Entry("new error without existing one", testCaseSetErrorMessage{
			errorType:            infrav1.RegistrationError, // errorType infrav1.ErrorType
			errorMessage:         "new message",             // errorMessage string
			hasErrorInStatus:     true,                      // hasErrorInStatus bool
			expectedErrorCount:   1,                         // expectedErrorCount int
			expectedErrorType:    infrav1.RegistrationError, //	expectedErrorType
			expectedErrorMessage: "new message",             // expectedErrorMessage
		}),
	)
})

var _ = Describe("test validateRootDeviceWwnsAreSubsetOfExistingWwns", func() {
	It("should return error when storageDevices is empty", func() {
		rootDeviceHints := &infrav1.RootDeviceHints{WWN: "wwn1"}
		storageDevices := []infrav1.Storage{}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).ToNot(BeNil())
		expectedError := fmt.Errorf(`%w for root device hint "wwn1". Known WWNs: []`, errMissingStorageDevice)
		Expect(err).To(Equal(expectedError))
	})
	It("should return nil when both rootDeviceHints and storageDevices are empty", func() {
		rootDeviceHints := &infrav1.RootDeviceHints{}
		storageDevices := []infrav1.Storage{}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).To(BeNil())
	})
	It("should return an error when rootDeviceHints contains WWNs not present in storageDevices", func() {
		rootDeviceHints := &infrav1.RootDeviceHints{WWN: "wwn3"}
		storageDevices := []infrav1.Storage{
			{WWN: "wwn1"},
			{WWN: "wwn2"},
		}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).NotTo(BeNil())
		expectedError := fmt.Errorf(`%w for root device hint "wwn3". Known WWNs: [wwn1 wwn2]`, errMissingStorageDevice)
		Expect(err).To(Equal(expectedError))
	})
	It("should return nil when rootDeviceHints contains WWNs present in storageDevices", func() {
		rootDeviceHints := &infrav1.RootDeviceHints{WWN: "wwn2"}
		storageDevices := []infrav1.Storage{
			{WWN: "wwn1"},
			{WWN: "wwn2"},
			{WWN: "wwn3"},
		}

		err := validateRootDeviceWwnsAreSubsetOfExistingWwns(rootDeviceHints, storageDevices)
		Expect(err).To(BeNil())
	})
})

var _ = Describe("obtainHardwareDetailsNics", func() {
	type testCaseObtainHardwareDetailsNics struct {
		stdout         string
		expectedOutput []infrav1.NIC
	}
	DescribeTable("Complete successfully",
		func(tc testCaseObtainHardwareDetailsNics) {
			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsNics").Return(sshclient.Output{StdOut: tc.stdout})

			Expect(obtainHardwareDetailsNics(sshMock)).Should(Equal(tc.expectedOutput))
		},
		Entry("proper response", testCaseObtainHardwareDetailsNics{
			stdout: `name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ip="23.88.6.239/26" speedMbps="1000"
	name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ip="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
			expectedOutput: []infrav1.NIC{
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
			},
		}),
	)
})

var _ = Describe("obtainHardwareDetailsStorage", func() {
	type testCaseObtainHardwareDetailsStorage struct {
		stdout               string
		expectedOutput       []infrav1.Storage
		expectedErrorMessage *string
	}
	DescribeTable("Complete successfully",
		func(tc testCaseObtainHardwareDetailsStorage) {
			sshMock := &sshmock.Client{}
			sshMock.On("GetHardwareDetailsStorage").Return(sshclient.Output{StdOut: tc.stdout})

			storageDevices, err := obtainHardwareDetailsStorage(sshMock)
			Expect(storageDevices).Should(Equal(tc.expectedOutput))
			if tc.expectedErrorMessage != nil {
				Expect(err.Error()).Should(ContainSubstring(*tc.expectedErrorMessage))
			} else {
				Expect(err).To(Succeed())
			}
		},
		Entry("proper response", testCaseObtainHardwareDetailsStorage{
			stdout: `NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			expectedOutput: []infrav1.Storage{
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
			expectedErrorMessage: nil,
		}),
		Entry("wrong rota", testCaseObtainHardwareDetailsStorage{
			stdout: `NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="2"
	NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
	NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			expectedOutput:       nil,
			expectedErrorMessage: ptr.To("unknown rota"),
		}),
	)
})

var _ = Describe("handleIncompleteBoot", func() {
	Context("correct hostname == rescue", func() {
		type testCaseHandleIncompleteBootCorrectHostname struct {
			isRebootIntoRescue    bool
			isTimeOut             bool
			isConnectionRefused   bool
			hostErrorType         infrav1.ErrorType
			expectedReturnError   error
			expectedHostErrorType infrav1.ErrorType
		}
		DescribeTable("hostName = rescue, varying error type and ssh client response - robot client giving all positive results, no timeouts",
			func(tc testCaseHandleIncompleteBootCorrectHostname) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(tc.hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, &robotMock, nil, nil, nil)

				if tc.expectedReturnError == nil {
					_, err := service.handleIncompleteBoot(tc.isRebootIntoRescue, tc.isTimeOut, tc.isConnectionRefused)
					Expect(err).To(Succeed())
				} else {
					_, err := service.handleIncompleteBoot(tc.isRebootIntoRescue, tc.isTimeOut, tc.isConnectionRefused)
					Expect(err).Should(Equal(tc.expectedReturnError))
				}

				Expect(host.Spec.Status.ErrorType).To(Equal(tc.expectedHostErrorType))
			},
			Entry("timeout, no errorType", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    true,
				isTimeOut:             true,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorType(""),
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSSHRebootTriggered,
			}),
			Entry("timeout,ErrorType == ErrorTypeSoftwareRebootTriggered", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    true,
				isTimeOut:             true,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorTypeSoftwareRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
			}),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootTriggered", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    true,
				isTimeOut:             true,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorTypeHardwareRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
			}),
			Entry("timeout,ErrorType == ErrorTypeSoftwareRebootTriggered", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    true,
				isTimeOut:             true,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorTypeSoftwareRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
			}),
			Entry("timeout,ErrorType == ErrorTypeHardwareRebootTriggered", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    true,
				isTimeOut:             true,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorTypeHardwareRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
			}),
			Entry("timeout,ErrorType == ErrorTypeSSHRebootTriggered", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    true,
				isTimeOut:             false,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorTypeSSHRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
			}),
			Entry("wrong boot", testCaseHandleIncompleteBootCorrectHostname{
				isRebootIntoRescue:    false,
				isTimeOut:             false,
				isConnectionRefused:   false,
				hostErrorType:         infrav1.ErrorType(""),
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
			}),
		)

		type testCaseHandleIncompleteBootDifferentResetTypes struct {
			isTimeOut             bool
			isConnectionRefused   bool
			rebootTypes           []infrav1.RebootType
			hostErrorType         infrav1.ErrorType
			expectedHostErrorType infrav1.ErrorType
			expectedRebootType    infrav1.RebootType
		}
		// Test with different reset type only software on machine
		DescribeTable("Different reset types",
			func(tc testCaseHandleIncompleteBootDifferentResetTypes) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					// Make sure that timeouts are exceeded to trigger escalation step
					helpers.WithError(tc.hostErrorType, "", 1, metav1.NewTime(time.Now().Add(-time.Hour))),
					helpers.WithRebootTypes(tc.rebootTypes),
				)
				service := newTestService(host, &robotMock, nil, nil, nil)

				_, err := service.handleIncompleteBoot(true, tc.isTimeOut, tc.isConnectionRefused)
				Expect(err).To(Succeed())
				Expect(host.Spec.Status.ErrorType).To(Equal(tc.expectedHostErrorType))
				if tc.expectedRebootType != infrav1.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", mock.Anything, tc.expectedRebootType)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
				}
			},
			Entry("timeout, no errorType, only hw reset", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:             true,
				isConnectionRefused:   false,
				rebootTypes:           []infrav1.RebootType{infrav1.RebootTypeHardware},
				hostErrorType:         infrav1.ErrorTypeSSHRebootTriggered,
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeHardware,
			}),
			Entry("wrong boot, only hw reset", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:             false,
				isConnectionRefused:   false,
				rebootTypes:           []infrav1.RebootType{infrav1.RebootTypeHardware},
				hostErrorType:         infrav1.ErrorType(""),
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeHardware,
			}),
			Entry("wrong boot, only hw reset, errorType =ErrorTypeSSHRebootTriggered", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:             false,
				isConnectionRefused:   false,
				rebootTypes:           []infrav1.RebootType{infrav1.RebootTypeHardware},
				hostErrorType:         infrav1.ErrorTypeSSHRebootTriggered,
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeHardware,
			}),
			Entry("wrong boot, errorType =ErrorTypeSSHRebootTriggered", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:             false,
				isConnectionRefused:   false,
				rebootTypes:           []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware},
				hostErrorType:         infrav1.ErrorTypeSSHRebootTriggered,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeSoftware,
			}),
			Entry("wrong boot,  errorType =ErrorTypeSoftwareRebootTriggered", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:             false,
				isConnectionRefused:   false,
				rebootTypes:           []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware},
				hostErrorType:         infrav1.ErrorTypeSoftwareRebootTriggered,
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeHardware,
			}),
			Entry("wrong boot,  errorType =ErrorTypeHardwareRebootTriggered", testCaseHandleIncompleteBootDifferentResetTypes{
				isTimeOut:             false,
				isConnectionRefused:   false,
				rebootTypes:           []infrav1.RebootType{infrav1.RebootTypeSoftware, infrav1.RebootTypeHardware},
				hostErrorType:         infrav1.ErrorTypeHardwareRebootTriggered,
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeHardware,
			}),
		)

		type testCaseHandleIncompleteBootDifferentTimeouts struct {
			hostErrorType         infrav1.ErrorType
			lastUpdated           time.Time
			expectedHostErrorType infrav1.ErrorType
			expectedRebootType    infrav1.RebootType
		}

		// Test with reached timeouts
		DescribeTable("Different timeouts",
			func(tc testCaseHandleIncompleteBootDifferentTimeouts) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(tc.hostErrorType, "", 1, metav1.Time{Time: tc.lastUpdated}),
				)
				service := newTestService(host, &robotMock, nil, nil, nil)

				_, err := service.handleIncompleteBoot(true, true, false)
				Expect(err).To(Succeed())
				Expect(host.Spec.Status.ErrorType).To(Equal(tc.expectedHostErrorType))
				if tc.expectedRebootType != infrav1.RebootType("") {
					Expect(robotMock.AssertCalled(GinkgoT(), "RebootBMServer", mock.Anything, tc.expectedRebootType)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
				}
			},
			Entry("timed out sw reset", testCaseHandleIncompleteBootDifferentTimeouts{
				hostErrorType:         infrav1.ErrorTypeSoftwareRebootTriggered,
				lastUpdated:           time.Now().Add(-5 * time.Minute),
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootTypeHardware,
			}),
			Entry("not timed out hw reset", testCaseHandleIncompleteBootDifferentTimeouts{
				hostErrorType:         infrav1.ErrorTypeHardwareRebootTriggered,
				lastUpdated:           time.Now().Add(-2 * time.Minute),
				expectedHostErrorType: infrav1.ErrorTypeHardwareRebootTriggered,
				expectedRebootType:    infrav1.RebootType(""),
			}),
			Entry("not timed out sw reset", testCaseHandleIncompleteBootDifferentTimeouts{
				hostErrorType:         infrav1.ErrorTypeSoftwareRebootTriggered,
				lastUpdated:           time.Now().Add(-3 * time.Minute),
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
				expectedRebootType:    infrav1.RebootType(""),
			}),
		)
		It("returns failed if connection error and timed out", func() {
			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

			host := helpers.BareMetalHost("test-host", "default",
				helpers.WithRebootTypes([]infrav1.RebootType{
					infrav1.RebootTypeSoftware,
					infrav1.RebootTypeHardware,
					infrav1.RebootTypePower,
				}),
				helpers.WithSSHSpec(),
				helpers.WithSSHStatus(),
				helpers.WithError(infrav1.ErrorTypeConnectionError, "", 1, metav1.Time{Time: time.Now().Add(-30 * time.Minute)}),
			)
			service := newTestService(host, &robotMock, nil, nil, nil)

			failed, err := service.handleIncompleteBoot(true, false, true)
			Expect(err).ToNot(BeNil())
			Expect(failed).To(BeTrue())
			Expect(host.Spec.Status.ErrorType).To(Equal(infrav1.ErrorTypeConnectionError))
			Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
		})

		It("fails if hardware reboot times out", func() {
			robotMock := robotmock.Client{}
			robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
			robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

			host := helpers.BareMetalHost("test-host", "default",
				helpers.WithRebootTypes([]infrav1.RebootType{
					infrav1.RebootTypeSoftware,
					infrav1.RebootTypeHardware,
					infrav1.RebootTypePower,
				}),
				helpers.WithSSHSpec(),
				helpers.WithSSHStatus(),
				helpers.WithError(infrav1.ErrorTypeHardwareRebootTriggered, "", 1, metav1.Time{Time: time.Now().Add(-time.Hour)}),
			)
			service := newTestService(host, &robotMock, nil, nil, nil)

			_, err := service.handleIncompleteBoot(true, true, false)
			Expect(err).ToNot(Succeed())
			Expect(host.Spec.Status.ErrorType).To(Equal(infrav1.ErrorTypeHardwareRebootTriggered))
			Expect(robotMock.AssertNotCalled(GinkgoT(), "RebootBMServer", mock.Anything, mock.Anything)).To(BeTrue())
		})
	})

	Context("hostname rescue vs machinename", func() {
		type testCaseHandleIncompleteBoot struct {
			isRebootIntoRescue    bool
			hostErrorType         infrav1.ErrorType
			expectedReturnError   error
			expectedHostErrorType infrav1.ErrorType
			expectsRescueCall     bool
		}

		DescribeTable("vary hostname and see whether rescue gets triggered",
			func(tc testCaseHandleIncompleteBoot) {
				robotMock := robotmock.Client{}
				robotMock.On("SetBootRescue", mock.Anything, sshFingerprint).Return(nil, nil)
				robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)
				robotMock.On("RebootBMServer", mock.Anything, mock.Anything).Return(nil, nil)

				host := helpers.BareMetalHost("test-host", "default",
					helpers.WithRebootTypes([]infrav1.RebootType{
						infrav1.RebootTypeSoftware,
						infrav1.RebootTypeHardware,
						infrav1.RebootTypePower,
					}),
					helpers.WithSSHSpec(),
					helpers.WithSSHStatus(),
					helpers.WithError(tc.hostErrorType, "", 1, metav1.Now()),
				)
				service := newTestService(host, &robotMock, nil, nil, nil)

				if tc.expectedReturnError == nil {
					_, err := service.handleIncompleteBoot(tc.isRebootIntoRescue, false, false)
					Expect(err).To(Succeed())
				} else {
					_, err := service.handleIncompleteBoot(tc.isRebootIntoRescue, false, false)
					Expect(err).Should(Equal(tc.expectedReturnError))
				}
				Expect(host.Spec.Status.ErrorType).To(Equal(tc.expectedHostErrorType))
				if tc.expectsRescueCall {
					Expect(robotMock.AssertCalled(GinkgoT(), "GetBootRescue", mock.Anything)).To(BeTrue())
				} else {
					Expect(robotMock.AssertNotCalled(GinkgoT(), "GetBootRescue", mock.Anything)).To(BeTrue())
				}
			},
			Entry("hostname == rescue", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:    true,
				hostErrorType:         infrav1.ErrorType(""),
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
				expectsRescueCall:     true,
			}),
			Entry("hostname != rescue", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:    false,
				hostErrorType:         infrav1.ErrorType(""),
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
				expectsRescueCall:     false,
			}),
			Entry("hostname == rescue, ErrType == ErrorTypeSSHRebootTriggered", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:    true,
				hostErrorType:         infrav1.ErrorTypeSSHRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
				expectsRescueCall:     true,
			}),
			Entry("hostname != rescue, ErrType == ErrorTypeSSHRebootTriggered", testCaseHandleIncompleteBoot{
				isRebootIntoRescue:    false,
				hostErrorType:         infrav1.ErrorTypeSSHRebootTriggered,
				expectedReturnError:   nil,
				expectedHostErrorType: infrav1.ErrorTypeSoftwareRebootTriggered,
				expectsRescueCall:     false,
			}),
		)
	})
})

var _ = Describe("ensureSSHKey", func() {
	defaultFingerPrint := "my-fingerprint"

	It("sets an error error if a key that exists under another name is uploaded", func() {
		secret := helpers.GetDefaultSSHSecret("ssh-secret", "default")
		robotMock := robotmock.Client{}
		sshSecretKeyRef := infrav1.SSHSecretKeyRef{
			Name:       "sshkey-name",
			PublicKey:  "public-key",
			PrivateKey: "private-key",
		}
		robotMock.On("SetSSHKey", string(secret.Data[sshSecretKeyRef.Name]), mock.Anything).Return(
			nil, models.Error{Code: models.ErrorCodeKeyAlreadyExists, Message: "key already exists"},
		)
		robotMock.On("ListSSHKeys").Return([]models.Key{
			{
				Name:        "secret2",
				Fingerprint: "my fingerprint",
			},
			{
				Name:        "secret3",
				Fingerprint: "my fingerprint",
			},
		}, nil)

		host := helpers.BareMetalHost("test-host", "default")

		service := newTestService(host, &robotMock, nil, nil, nil)

		sshKey, actResult := service.ensureSSHKey(infrav1.SSHSecretRef{
			Name: "secret-name",
			Key:  sshSecretKeyRef,
		}, secret)

		emptySSHKey := infrav1.SSHKey{}
		Expect(sshKey).To(Equal(emptySSHKey))
		Expect(actResult).To(BeAssignableToTypeOf(actionFailed{}))
		Expect(host.Spec.Status.ErrorType).To(Equal(infrav1.PreparationError))
	})

	type testCaseEnsureSSHKey struct {
		hetznerSSHKeys       []models.Key
		sshSecretKeyRef      infrav1.SSHSecretKeyRef
		expectedFingerprint  string
		expectedActionResult actionResult
		expectSetSSHKey      bool
	}

	DescribeTable("ensureSSHKey",
		func(tc testCaseEnsureSSHKey) {
			secret := helpers.GetDefaultSSHSecret("ssh-secret", "default")
			robotMock := robotmock.Client{}
			robotMock.On("SetSSHKey", string(secret.Data[tc.sshSecretKeyRef.Name]), mock.Anything).Return(
				&models.Key{Name: tc.sshSecretKeyRef.Name, Fingerprint: defaultFingerPrint}, nil,
			)
			robotMock.On("ListSSHKeys").Return(tc.hetznerSSHKeys, nil)

			host := helpers.BareMetalHost("test-host", "default")

			service := newTestService(host, &robotMock, nil, nil, nil)

			sshKey, actResult := service.ensureSSHKey(infrav1.SSHSecretRef{
				Name: "secret-name",
				Key:  tc.sshSecretKeyRef,
			}, secret)

			Expect(sshKey.Fingerprint).To(Equal(tc.expectedFingerprint))
			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
			if tc.expectSetSSHKey {
				Expect(robotMock.AssertCalled(GinkgoT(), "SetSSHKey", string(secret.Data[tc.sshSecretKeyRef.Name]), mock.Anything)).To(BeTrue())
			} else {
				Expect(robotMock.AssertNotCalled(GinkgoT(), "SetSSHKey", string(secret.Data[tc.sshSecretKeyRef.Name]), mock.Anything)).To(BeTrue())
			}
		},
		Entry("empty list", testCaseEnsureSSHKey{
			hetznerSSHKeys: nil,
			sshSecretKeyRef: infrav1.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			expectedFingerprint:  defaultFingerPrint,
			expectedActionResult: actionComplete{},
			expectSetSSHKey:      true,
		}),
		Entry("secret in list", testCaseEnsureSSHKey{
			hetznerSSHKeys: []models.Key{
				{
					Name:        "my-name",
					Fingerprint: "my-fingerprint",
				},
			},
			sshSecretKeyRef: infrav1.SSHSecretKeyRef{
				Name:       "sshkey-name",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
			},
			expectedFingerprint:  "my-fingerprint",
			expectedActionResult: actionComplete{},
			expectSetSSHKey:      false,
		}),
		Entry(
			"secret not in list", testCaseEnsureSSHKey{
				hetznerSSHKeys: []models.Key{
					{
						Name:        "secret2",
						Fingerprint: "my fingerprint",
					},
					{
						Name:        "secret3",
						Fingerprint: "my fingerprint",
					},
				},
				sshSecretKeyRef: infrav1.SSHSecretKeyRef{
					Name:       "sshkey-name",
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				},
				expectedFingerprint:  defaultFingerPrint,
				expectedActionResult: actionComplete{},
				expectSetSSHKey:      true,
			}),
	)
})

var _ = Describe("analyzeSSHOutputInstallImage", func() {
	type testCaseAnalyzeSSHOutputInstallImageOutErr struct {
		err                         error
		rescueActive                bool
		expectedIsTimeout           bool
		expectedIsConnectionRefused bool
		expectedErrMessage          string
	}

	DescribeTable("analyzeSSHOutputInstallImage - out.Err",
		func(tc testCaseAnalyzeSSHOutputInstallImageOutErr) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
			)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: tc.rescueActive}, nil)

			service := newTestService(host, &robotMock, nil, nil, nil)

			isTimeout, isConnectionRefused, err := service.analyzeSSHOutputRegistering(sshclient.Output{Err: tc.err})
			Expect(isTimeout).To(Equal(tc.expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(tc.expectedIsConnectionRefused))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("timeout error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         timeout,
			rescueActive:                true,
			expectedIsTimeout:           true,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, rescue active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			rescueActive:                true,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, rescue not active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			rescueActive:                false,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "wrong ssh key",
		}),
		Entry("connectionRefused error, rescue active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrConnectionRefused,
			rescueActive:                true,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
		Entry("connectionRefused error, rescue not active", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrConnectionRefused,
			rescueActive:                false,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
	)

	type testCaseAnalyzeSSHOutputInstallImageStdErr struct {
		hasNilErr          bool
		stdErr             string
		hostName           string
		expectedErrMessage string
	}

	DescribeTable("analyzeSSHOutputRegistering - toggle stdErr and hostName",
		func(tc testCaseAnalyzeSSHOutputInstallImageStdErr) {
			var err error
			if !tc.hasNilErr {
				err = errTest
			}

			out := sshclient.Output{
				StdOut: tc.hostName,
				StdErr: tc.stdErr,
				Err:    err,
			}

			host := helpers.BareMetalHost(
				"test-host",
				"default",
			)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: true}, nil)

			service := newTestService(host, &robotMock, nil, nil, nil)

			isTimeout, isConnectionRefused, err := service.analyzeSSHOutputRegistering(out)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("stderr not empty", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hostName:           "hostName",
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("stderr not empty - err != nil", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          false,
			stdErr:             "command failed",
			hostName:           "",
			expectedErrMessage: "unhandled ssh error while getting hostname",
		}),
		Entry("stderr not empty - wrong hostName", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hostName:           "",
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("stderr empty - wrong hostName", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "",
			expectedErrMessage: "hostname is empty",
		}),
	)
})

var _ = Describe("analyzeSSHOutputInstallImage", func() {
	type testCaseAnalyzeSSHOutputInstallImageOutErr struct {
		err                         error
		errFromGetHostNameNil       bool
		port                        int
		expectedIsTimeout           bool
		expectedIsConnectionRefused bool
		expectedErrMessage          string
	}

	DescribeTable("analyzeSSHOutputInstallImage - out.Err",
		func(tc testCaseAnalyzeSSHOutputInstallImageOutErr) {
			sshMock := &sshmock.Client{}
			var errFromGetHostName error
			if !tc.errFromGetHostNameNil {
				errFromGetHostName = errTest
			}
			sshMock.On("GetHostName").Return(sshclient.Output{Err: errFromGetHostName})

			isTimeout, isConnectionRefused, err := analyzeSSHOutputInstallImage(sshclient.Output{Err: tc.err}, sshMock, tc.port)
			Expect(isTimeout).To(Equal(tc.expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(tc.expectedIsConnectionRefused))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("timeout error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         timeout,
			errFromGetHostNameNil:       true,
			port:                        22,
			expectedIsTimeout:           true,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, port 22, no hostName error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			errFromGetHostNameNil:       true,
			port:                        22,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
		Entry("authenticationFailed error, port 22, hostName error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			errFromGetHostNameNil:       false,
			port:                        22,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "wrong ssh key",
		}),
		Entry("authenticationFailed error, port != 22", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrAuthenticationFailed,
			errFromGetHostNameNil:       true,
			port:                        23,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "wrong ssh key",
		}),
		Entry("connectionRefused error, port 22", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrConnectionRefused,
			errFromGetHostNameNil:       true,
			port:                        22,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
		Entry("connectionRefused error, port != 22, hostname error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrConnectionRefused,
			errFromGetHostNameNil:       false,
			port:                        23,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          "",
		}),
		Entry("connectionRefused error, port != 22, no hostname error", testCaseAnalyzeSSHOutputInstallImageOutErr{
			err:                         sshclient.ErrConnectionRefused,
			errFromGetHostNameNil:       true,
			port:                        23,
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          "",
		}),
	)

	type testCaseAnalyzeSSHOutputInstallImageStdErr struct {
		hasNilErr          bool
		stdErr             string
		hasWrongHostName   bool
		expectedErrMessage string
	}

	DescribeTable("analyzeSSHOutputInstallImage - StdErr not empty",
		func(tc testCaseAnalyzeSSHOutputInstallImageStdErr) {
			var err error
			if !tc.hasNilErr {
				err = errTest
			}
			hostName := "rescue"
			if tc.hasWrongHostName {
				hostName = "wrongHostName"
			}

			out := sshclient.Output{
				StdOut: hostName,
				StdErr: tc.stdErr,
				Err:    err,
			}
			isTimeout, isConnectionRefused, err := analyzeSSHOutputInstallImage(out, nil, 22)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("stderr not empty", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hasWrongHostName:   false,
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("stderr not empty - err != nil", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          false,
			stdErr:             "command failed",
			hasWrongHostName:   false,
			expectedErrMessage: "unhandled ssh error while getting hostname",
		}),
		Entry("stderr not empty - wrong hostName", testCaseAnalyzeSSHOutputInstallImageStdErr{
			hasNilErr:          true,
			stdErr:             "command failed",
			hasWrongHostName:   true,
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
	)

	type testCaseAnalyzeSSHOutputInstallImageWrongHostname struct {
		hasNilErr          bool
		stdErr             string
		hostName           string
		expectedErrMessage string
	}

	DescribeTable("analyzeSSHOutputInstallImage - wrong hostName",
		func(tc testCaseAnalyzeSSHOutputInstallImageWrongHostname) {
			var err error
			if !tc.hasNilErr {
				err = errTest
			}

			out := sshclient.Output{
				StdOut: tc.hostName,
				StdErr: tc.stdErr,
				Err:    err,
			}
			isTimeout, isConnectionRefused, err := analyzeSSHOutputInstallImage(out, nil, 22)
			Expect(isTimeout).To(Equal(false))
			Expect(isConnectionRefused).To(Equal(false))
			if tc.expectedErrMessage != "" {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("empty hostname", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "",
			expectedErrMessage: "hostname is empty",
		}),
		Entry("empty hostname - err not empty", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          false,
			stdErr:             "",
			hostName:           "",
			expectedErrMessage: "unhandled ssh error while getting hostname",
		}),
		Entry("empty hostname stderr not empty", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "command failed",
			hostName:           "",
			expectedErrMessage: "failed to get hostname via ssh: StdErr:",
		}),
		Entry("hostname == rescue", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "rescue",
			expectedErrMessage: "",
		}),
		Entry("hostname == otherHostName", testCaseAnalyzeSSHOutputInstallImageWrongHostname{
			hasNilErr:          true,
			stdErr:             "",
			hostName:           "otherHostName",
			expectedErrMessage: "unexpected hostname",
		}),
	)
})

var _ = Describe("analyzeSSHOutputProvisioned", func() {
	type testCaseAnalyzeSSHOutputProvisioned struct {
		out                         sshclient.Output
		expectedIsTimeout           bool
		expectedIsConnectionRefused bool
		expectedErrMessage          *string
	}

	DescribeTable("analyzeSSHOutputProvisioned",
		func(tc testCaseAnalyzeSSHOutputProvisioned) {
			isTimeout, isConnectionRefused, err := analyzeSSHOutputProvisioned(tc.out)
			Expect(isTimeout).To(Equal(tc.expectedIsTimeout))
			Expect(isConnectionRefused).To(Equal(tc.expectedIsConnectionRefused))
			if tc.expectedErrMessage != nil {
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(ContainSubstring(*tc.expectedErrMessage))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("incorrect boot", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{StdOut: "wrong_hostname"},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("unexpected hostname"),
		}),
		Entry("timeout error", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{Err: timeout},
			expectedIsTimeout:           true,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          nil,
		}),
		Entry("stdErr non-empty", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{StdErr: "some error"},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("failed to get hostname via ssh: StdErr: some error"),
		}),
		Entry("incorrect boot - empty hostname", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{StdOut: ""},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("hostname is empty"),
		}),
		Entry("unable to authenticate", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{Err: sshclient.ErrAuthenticationFailed},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: false,
			expectedErrMessage:          ptr.To("wrong ssh key"),
		}),
		Entry("connection refused", testCaseAnalyzeSSHOutputProvisioned{
			out:                         sshclient.Output{Err: sshclient.ErrConnectionRefused},
			expectedIsTimeout:           false,
			expectedIsConnectionRefused: true,
			expectedErrMessage:          nil,
		}),
	)
})

var _ = Describe("actionRegistering", func() {
	type testCaseActionRegistering struct {
		storageStdOut             string
		includeRootDeviceHintWWN  bool
		includeRootDeviceHintRaid bool
		expectedActionResult      actionResult
		expectedErrorMessage      *string
		swRaid                    bool
	}

	DescribeTable("actionRegistering",
		func(tc testCaseActionRegistering) {
			var host *infrav1.HetznerBareMetalHost
			if tc.includeRootDeviceHintWWN {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithRootDeviceHintWWN(),
					helpers.WithIPv4(),
					helpers.WithConsumerRef(),
				)
			} else if tc.includeRootDeviceHintRaid {
				host = helpers.BareMetalHost(
					"test-host",
					"default",
					helpers.WithRootDeviceHintRaid(),
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
			host.Spec.Status.InstallImage = &infrav1.InstallImage{}
			if tc.swRaid {
				host.Spec.Status.InstallImage.Swraid = 1
			}
			sshMock := registeringSSHMock(tc.storageStdOut)
			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionRegistering()
			Expect(host.Spec.Status.HardwareDetails).ToNot(BeNil())
			if tc.expectedErrorMessage != nil {
				Expect(host.Spec.Status.ErrorMessage).To(Equal(*tc.expectedErrorMessage))
			}
			switch tc.expectedActionResult.(type) {
			case actionComplete:
				Expect(host.Spec.Status.ErrorMessage).To(Equal(""))
			case *actionContinue:
				Expect(host.Spec.Status.ErrorMessage).To(Equal(""))
			}
			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
		},
		Entry("working example", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
		NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
		NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  true,
			includeRootDeviceHintRaid: false,
			expectedActionResult:      actionComplete{},
			expectedErrorMessage:      nil,
		}),
		Entry("working example - rootDeviceHints raid", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
		NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
		NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  false,
			includeRootDeviceHintRaid: true,
			expectedActionResult:      actionComplete{},
			expectedErrorMessage:      nil,
			swRaid:                    true,
		}),
		Entry("wwn does not fit to storage devices", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
			NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
			NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  true,
			includeRootDeviceHintRaid: false,
			expectedActionResult:      actionFailed{},
			expectedErrorMessage:      ptr.To(`missing storage device for root device hint "eui.002538b411b2cee8". Known WWNs: [eui.002538b411b2cee2 eui.0025388801b4dff2]`),
		}),
		Entry("no root device hints", testCaseActionRegistering{
			storageStdOut: `NAME="loop0" LABEL="" FSTYPE="ext2" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
			NAME="nvme2n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee2" ROTA="0"
			NAME="nvme1n1" LABEL="" FSTYPE="" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			includeRootDeviceHintWWN:  false,
			includeRootDeviceHintRaid: false,
			expectedActionResult:      actionFailed{},
			expectedErrorMessage:      ptr.To(infrav1.ErrorMessageMissingRootDeviceHints),
		}),
	)

	type testCaseActionRegisteringIncompleteBoot struct {
		getHostNameOutput sshclient.Output
		expectedErrorType infrav1.ErrorType
	}

	DescribeTable("actionRegistering - incomplete reboot",
		func(tc testCaseActionRegisteringIncompleteBoot) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithRebootTypes([]infrav1.RebootType{infrav1.RebootTypeHardware}),
				helpers.WithRootDeviceHintWWN(),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName").Return(tc.getHostNameOutput)

			robotMock := robotmock.Client{}
			robotMock.On("GetBootRescue", mock.Anything).Return(&models.Rescue{Active: false}, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionRegistering()
			Expect(actResult).Should(BeAssignableToTypeOf(actionContinue{}))
			if tc.expectedErrorType != infrav1.ErrorType("") {
				Expect(host.Spec.Status.ErrorType).To(Equal(tc.expectedErrorType))
			}
		},
		Entry("timeout", testCaseActionRegisteringIncompleteBoot{
			getHostNameOutput: sshclient.Output{Err: timeout},
			expectedErrorType: infrav1.ErrorTypeSSHRebootTriggered,
		}),
		Entry("connectionRefused", testCaseActionRegisteringIncompleteBoot{
			getHostNameOutput: sshclient.Output{Err: sshclient.ErrConnectionRefused},
			expectedErrorType: infrav1.ErrorTypeConnectionError,
		}),
	)
})

func registeringSSHMock(storageStdOut string) *sshmock.Client {
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
	sshMock.On("GetHardwareDetailsDebug").Return(sshclient.Output{StdOut: "Dummy output"})
	return sshMock
}

var _ = Describe("actionRegistering check RAID", func() {
	It("check RAID", func() {
		sshMock := registeringSSHMock(`NAME="nvme2n1" TYPE="disk" MODEL="mymode." VENDOR="" SIZE="3068773888" WWN="wwn1" ROTA="0"
		NAME="nvme2n2" TYPE="disk" MODEL="mymodel" VENDOR="" SIZE="3068773888" WWN="wwn2" ROTA="0"`)
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithConsumerRef(),
		)
		host.Spec.RootDeviceHints.WWN = "wwn1"
		host.Spec.Status.InstallImage = &infrav1.InstallImage{
			Swraid: 1,
		}
		service := newTestService(host, nil, bmmock.NewSSHFactory(
			sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		actResult := service.actionRegistering()

		_, err := actResult.Result()
		Expect(err).Should(BeNil())
		Expect(host.Spec.Status.ErrorMessage).Should(Equal("Invalid HetznerBareMetalHost: spec.status.installImage.swraid is active. Use at least two WWNs in spec.rootDevideHints.raid.wwn."))

		host.Spec.Status.InstallImage.Swraid = 0
		host.Spec.RootDeviceHints.WWN = ""
		host.Spec.RootDeviceHints.Raid.WWN = []string{"wwn1", "wwn2"}
		actResult = service.actionRegistering()

		_, err = actResult.Result()
		Expect(err).Should(BeNil())
		Expect(host.Spec.Status.ErrorMessage).Should(Equal("Invalid HetznerBareMetalHost: spec.status.installImage.swraid is not active. Use spec.rootDevideHints.wwn and leave raid.wwn empty."))
	})
})

var _ = Describe("getImageDetails", func() {
	type testCaseGetImageDetails struct {
		image                 infrav1.Image
		expectedImagePath     string
		expectedNeedsDownload bool
		expectedErrorMessage  string
	}

	DescribeTable("getImageDetails",
		func(tc testCaseGetImageDetails) {
			imagePath, needsDownload, errorMessage := tc.image.GetDetails()
			Expect(imagePath).To(Equal(tc.expectedImagePath))
			Expect(needsDownload).To(Equal(tc.expectedNeedsDownload))
			Expect(errorMessage).To(Equal(tc.expectedErrorMessage))
		},
		Entry("name and url specified, tar.gz suffix", testCaseGetImageDetails{
			image: infrav1.Image{
				Name: "imageName",
				URL:  "https://mytargz.tar.gz",
				Path: "",
			},
			expectedImagePath:     "/root/imageName.tar.gz",
			expectedNeedsDownload: true,
			expectedErrorMessage:  "",
		}),
		Entry("name and url specified, tgz suffix", testCaseGetImageDetails{
			image: infrav1.Image{
				Name: "imageName",
				URL:  "https://mytargz.tgz",
				Path: "",
			},
			expectedImagePath:     "/root/imageName.tgz",
			expectedNeedsDownload: true,
			expectedErrorMessage:  "",
		}),
		Entry("name and url specified, wrong suffix", testCaseGetImageDetails{
			image: infrav1.Image{
				Name: "imageName",
				URL:  "https://mytargz.tgx",
				Path: "",
			},
			expectedImagePath:     "",
			expectedNeedsDownload: false,
			expectedErrorMessage:  "wrong image url suffix",
		}),
		Entry("path specified", testCaseGetImageDetails{
			image: infrav1.Image{
				Name: "",
				URL:  "",
				Path: "imagePath",
			},
			expectedImagePath:     "imagePath",
			expectedNeedsDownload: false,
			expectedErrorMessage:  "",
		}),
		Entry("neither specified", testCaseGetImageDetails{
			image: infrav1.Image{
				Name: "imageName",
				URL:  "",
				Path: "",
			},
			expectedImagePath:     "",
			expectedNeedsDownload: false,
			expectedErrorMessage:  "invalid image - need to specify either name and url or path",
		}),
	)
})

var _ = Describe("actionEnsureProvisioned", func() {
	type testCaseActionEnsureProvisioned struct {
		outSSHClientGetHostName                sshclient.Output
		outSSHClientCloudInitStatus            sshclient.Output
		outSSHClientCheckSigterm               sshclient.Output
		outOldSSHClientCloudInitStatus         sshclient.Output
		outOldSSHClientCheckSigterm            sshclient.Output
		samePorts                              bool
		expectedActionResult                   actionResult
		expectedErrorType                      infrav1.ErrorType
		expectsSSHClientCallCloudInitStatus    bool
		expectsSSHClientCallCheckSigterm       bool
		expectsSSHClientCallReboot             bool
		expectsOldSSHClientCallCloudInitStatus bool
		expectsOldSSHClientCallCheckSigterm    bool
		expectsOldSSHClientCallReboot          bool
	}

	DescribeTable("actionEnsureProvisioned",
		func(in testCaseActionEnsureProvisioned) {
			var (
				portAfterCloudInit    = 24
				portAfterInstallImage = 23
			)
			if in.samePorts {
				portAfterInstallImage = 24
			}

			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHSpecInclPorts(portAfterInstallImage, portAfterCloudInit),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			sshMock := &sshmock.Client{}
			sshMock.On("GetHostName").Return(in.outSSHClientGetHostName)
			sshMock.On("CloudInitStatus").Return(in.outSSHClientCloudInitStatus)
			sshMock.On("CheckCloudInitLogsForSigTerm").Return(in.outSSHClientCheckSigterm)
			sshMock.On("CleanCloudInitLogs").Return(sshclient.Output{})
			sshMock.On("CleanCloudInitInstances").Return(sshclient.Output{})
			sshMock.On("Reboot").Return(sshclient.Output{})
			sshMock.On("GetCloudInitOutput").Return(sshclient.Output{StdOut: "dummy content of /var/log/cloud-init-output.log"})

			oldSSHMock := &sshmock.Client{}
			oldSSHMock.On("CloudInitStatus").Return(in.outOldSSHClientCloudInitStatus)
			oldSSHMock.On("CheckCloudInitLogsForSigTerm").Return(in.outOldSSHClientCheckSigterm)
			oldSSHMock.On("CleanCloudInitLogs").Return(sshclient.Output{})
			oldSSHMock.On("CleanCloudInitInstances").Return(sshclient.Output{})
			oldSSHMock.On("Reboot").Return(sshclient.Output{})

			robotMock := robotmock.Client{}
			robotMock.On("SetBMServerName", mock.Anything, infrav1.BareMetalHostNamePrefix+host.Spec.ConsumerRef.Name).Return(nil, nil)

			service := newTestService(host, &robotMock, bmmock.NewSSHFactory(sshMock, oldSSHMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), nil)

			actResult := service.actionEnsureProvisioned()
			Expect(actResult).Should(BeAssignableToTypeOf(in.expectedActionResult))
			if in.expectedErrorType != infrav1.ErrorType("") {
				Expect(host.Spec.Status.ErrorType).To(Equal(in.expectedErrorType))
			}
			if in.expectsSSHClientCallCloudInitStatus {
				Expect(sshMock.AssertCalled(GinkgoT(), "CloudInitStatus")).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "CloudInitStatus")).To(BeTrue())
			}
			if in.expectsSSHClientCallCheckSigterm {
				Expect(sshMock.AssertCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm")).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm")).To(BeTrue())
			}
			if in.expectsSSHClientCallReboot {
				Expect(sshMock.AssertCalled(GinkgoT(), "Reboot")).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "Reboot")).To(BeTrue())
			}
			if in.expectsOldSSHClientCallCloudInitStatus {
				Expect(oldSSHMock.AssertCalled(GinkgoT(), "CloudInitStatus")).To(BeTrue())
			} else {
				Expect(oldSSHMock.AssertNotCalled(GinkgoT(), "CloudInitStatus")).To(BeTrue())
			}
			if in.expectsOldSSHClientCallCheckSigterm {
				Expect(oldSSHMock.AssertCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm")).To(BeTrue())
			} else {
				Expect(oldSSHMock.AssertNotCalled(GinkgoT(), "CheckCloudInitLogsForSigTerm")).To(BeTrue())
			}
			if in.expectsOldSSHClientCallReboot {
				Expect(oldSSHMock.AssertCalled(GinkgoT(), "Reboot")).To(BeTrue())
			} else {
				Expect(oldSSHMock.AssertNotCalled(GinkgoT(), "Reboot")).To(BeTrue())
			}
		},
		Entry("correct hostname, cloud init running",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: running"},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              true,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init done, no SIGTERM",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: done"},
				outSSHClientCheckSigterm:               sshclient.Output{StdOut: ""},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              true,
				expectedActionResult:                   actionComplete{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       true,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init done, SIGTERM",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: done"},
				outSSHClientCheckSigterm:               sshclient.Output{StdOut: "found SIGTERM in cloud init output logs"},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              true,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       true,
				expectsSSHClientCallReboot:             true,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init error",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: error"},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              true,
				expectedActionResult:                   actionFailed{},
				expectedErrorType:                      infrav1.FatalError,
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init disabled",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: disabled"},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              true,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             true,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, same ports",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              true,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorTypeConnectionError,
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, different ports, connectionFailed of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorTypeConnectionError,
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, different ports, status running of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{StdOut: "status: running"},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, different ports, status error of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{StdOut: "status: error"},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionFailed{},
				expectedErrorType:                      infrav1.FatalError,
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, different ports, status disabled of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{StdOut: "status: disabled"},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          true,
			},
		),
		Entry("connectionFailed, different ports, status done of oldSSHClient, SIGTERM of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{StdOut: "status: done"},
				outOldSSHClientCheckSigterm:            sshclient.Output{StdOut: "found SIGTERM in cloud init output logs"},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    true,
				expectsOldSSHClientCallReboot:          true,
			},
		),
		Entry("connectionFailed, different ports, status done of oldSSHClient, no SIGTERM of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{StdOut: "status: done"},
				outOldSSHClientCheckSigterm:            sshclient.Output{StdOut: ""},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    true,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("connectionFailed, different ports, timeout of oldSSHClient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: sshclient.ErrConnectionRefused},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{Err: timeout},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorTypeConnectionError,
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: true,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("correct hostname, cloud init done, no SIGTERM, ports different",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + "bm-machine"},
				outSSHClientCloudInitStatus:            sshclient.Output{StdOut: "status: done"},
				outSSHClientCheckSigterm:               sshclient.Output{StdOut: ""},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionComplete{},
				expectedErrorType:                      infrav1.ErrorType(""),
				expectsSSHClientCallCloudInitStatus:    true,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
		Entry("timeout of sshclient",
			testCaseActionEnsureProvisioned{
				outSSHClientGetHostName:                sshclient.Output{Err: timeout},
				outSSHClientCloudInitStatus:            sshclient.Output{},
				outSSHClientCheckSigterm:               sshclient.Output{},
				outOldSSHClientCloudInitStatus:         sshclient.Output{},
				outOldSSHClientCheckSigterm:            sshclient.Output{},
				samePorts:                              false,
				expectedActionResult:                   actionContinue{},
				expectedErrorType:                      infrav1.ErrorTypeSSHRebootTriggered,
				expectsSSHClientCallCloudInitStatus:    false,
				expectsSSHClientCallCheckSigterm:       false,
				expectsSSHClientCallReboot:             false,
				expectsOldSSHClientCallCloudInitStatus: false,
				expectsOldSSHClientCallCheckSigterm:    false,
				expectsOldSSHClientCallReboot:          false,
			},
		),
	)
})

var _ = Describe("actionProvisioned", func() {
	type testCaseActionProvisioned struct {
		shouldHaveRebootAnnotation bool
		rebooted                   bool
		rebootFinished             bool
		expectedActionResult       actionResult
		expectRebootAnnotation     bool
		expectRebootInStatus       bool
	}

	DescribeTable("actionProvisioned",
		func(tc testCaseActionProvisioned) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHSpecInclPorts(23, 24),
				helpers.WithIPv4(),
				helpers.WithConsumerRef(),
			)

			if tc.shouldHaveRebootAnnotation {
				host.SetAnnotations(map[string]string{infrav1.RebootAnnotation: "reboot"})
			}

			host.Spec.Status.Rebooted = tc.rebooted

			sshMock := &sshmock.Client{}
			var hostNameOutput sshclient.Output
			if tc.rebootFinished {
				hostNameOutput = sshclient.Output{StdOut: infrav1.BareMetalHostNamePrefix + host.Spec.ConsumerRef.Name}
			} else {
				hostNameOutput = sshclient.Output{Err: timeout}
			}
			sshMock.On("GetHostName").Return(hostNameOutput)
			sshMock.On("Reboot").Return(sshclient.Output{})

			service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), helpers.GetDefaultSSHSecret(osSSHKeyName, "default"), helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))

			actResult := service.actionProvisioned()
			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
			Expect(host.Spec.Status.Rebooted).To(Equal(tc.expectRebootInStatus))
			Expect(host.HasRebootAnnotation()).To(Equal(tc.expectRebootAnnotation))

			if tc.shouldHaveRebootAnnotation && !tc.rebooted {
				Expect(sshMock.AssertCalled(GinkgoT(), "Reboot")).To(BeTrue())
			} else {
				Expect(sshMock.AssertNotCalled(GinkgoT(), "Reboot")).To(BeTrue())
			}
		},
		Entry("reboot desired, but not performed yet", testCaseActionProvisioned{
			shouldHaveRebootAnnotation: true,
			rebooted:                   false,
			rebootFinished:             false,
			expectedActionResult:       actionContinue{},
			expectRebootAnnotation:     true,
			expectRebootInStatus:       true,
		}),
		Entry("reboot desired, and already performed, not finished", testCaseActionProvisioned{
			shouldHaveRebootAnnotation: true,
			rebooted:                   true,
			rebootFinished:             false,
			expectedActionResult:       actionContinue{},
			expectRebootAnnotation:     true,
			expectRebootInStatus:       true,
		}),
		Entry("reboot desired, and already performed, finished", testCaseActionProvisioned{
			shouldHaveRebootAnnotation: true,
			rebooted:                   true,
			rebootFinished:             true,
			expectedActionResult:       actionComplete{},
			expectRebootAnnotation:     false,
			expectRebootInStatus:       false,
		}),
		Entry("no reboot desired", testCaseActionProvisioned{
			shouldHaveRebootAnnotation: false,
			rebooted:                   false,
			rebootFinished:             false,
			expectedActionResult:       actionComplete{},
			expectRebootAnnotation:     false,
			expectRebootInStatus:       false,
		}),
	)
})
