/*
Copyright 2023 The Kubernetes Authors.

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

package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Test update secret status", func() {
	host := HetznerBareMetalHost{}
	secret := corev1.Secret{}
	secret.Name = "secret_name"
	secret.Namespace = "secret_namespace"
	secret.Data = map[string][]byte{"key1": []byte("val1"), "key2": []byte("val2")}

	hash, err := HashOfSecretData(secret.Data)
	Expect(err).To(BeNil())

	Context("Test UpdateRescueSSHStatus", func() {
		err = host.UpdateRescueSSHStatus(secret)
		Expect(err).To(BeNil())
		Expect(host.Spec.Status.SSHStatus.CurrentRescue.Reference.Name).Should(Equal(secret.Name))
		Expect(host.Spec.Status.SSHStatus.CurrentRescue.Reference.Namespace).Should(Equal(secret.Namespace))
		Expect(host.Spec.Status.SSHStatus.CurrentRescue.DataHash).Should(Equal(hash))
	})

	Context("Test UpdateOSSSHStatus", func() {
		err = host.UpdateOSSSHStatus(secret)
		Expect(err).To(BeNil())

		Expect(host.Spec.Status.SSHStatus.CurrentOS.Reference.Name).Should(Equal(secret.Name))
		Expect(host.Spec.Status.SSHStatus.CurrentOS.Reference.Namespace).Should(Equal(secret.Namespace))
		Expect(host.Spec.Status.SSHStatus.CurrentOS.DataHash).Should(Equal(hash))
	})

	Context("Test statusFromSecret", func() {
		status, err := statusFromSecret(secret)
		Expect(err).To(BeNil())

		Expect(status.Reference.Name).Should(Equal(secret.Name))
		Expect(status.Reference.Namespace).Should(Equal(secret.Namespace))
		Expect(status.DataHash).Should(Equal(hash))
	})

	Context("Test secretStatus.Match", func() {
		It("returns false when secretStatus is nil", func() {
			status := SecretStatus{}
			Expect(status.Match(secret)).To(BeFalse())
		})

		type testCaseSecretStatusMatch struct {
			name       string
			namespace  string
			data       map[string][]byte
			expectBool bool
		}

		DescribeTable("Test secretStatus.Match",
			func(tc testCaseSecretStatusMatch) {
				status := SecretStatus{Reference: &corev1.SecretReference{Name: tc.name, Namespace: tc.namespace}}

				hash, err := HashOfSecretData(tc.data)
				Expect(err).To(BeNil())

				status.DataHash = hash

				Expect(status.Match(secret)).Should(Equal(tc.expectBool))
			},
			Entry("equal", testCaseSecretStatusMatch{
				name:       secret.Name,
				namespace:  secret.Namespace,
				data:       secret.Data,
				expectBool: true,
			}),
			Entry("wrong name", testCaseSecretStatusMatch{
				name:       "other_name",
				namespace:  secret.Namespace,
				data:       secret.Data,
				expectBool: false,
			}),
			Entry("wrong namespace", testCaseSecretStatusMatch{
				name:       secret.Name,
				namespace:  "other_namespace",
				data:       secret.Data,
				expectBool: false,
			}),
			Entry("wrong data", testCaseSecretStatusMatch{
				name:       secret.Name,
				namespace:  secret.Namespace,
				data:       map[string][]byte{"other": []byte("data")},
				expectBool: false,
			}),
		)
	})
})

var _ = Describe("Test RootDeviceHints.IsValid", func() {
	type testCaseRootDeviceHintsIsValid struct {
		wwn        string
		raid       Raid
		expectBool bool
	}

	DescribeTable("Test RootDeviceHints.IsValid",
		func(tc testCaseRootDeviceHintsIsValid) {
			rdh := RootDeviceHints{
				WWN:  tc.wwn,
				Raid: tc.raid,
			}

			Expect(rdh.IsValid()).Should(Equal(tc.expectBool))
		},
		Entry("wwn set", testCaseRootDeviceHintsIsValid{
			wwn:        "test-wwn",
			raid:       Raid{},
			expectBool: true,
		}),
		Entry("raid set", testCaseRootDeviceHintsIsValid{
			wwn:        "",
			raid:       Raid{WWN: []string{"test-wwn1", "test-wwn2"}},
			expectBool: true,
		}),
		Entry("wwn and raid set", testCaseRootDeviceHintsIsValid{
			wwn:        "test-wwn",
			raid:       Raid{WWN: []string{"test-wwn1", "test-wwn2"}},
			expectBool: false,
		}),
		Entry("nothing set", testCaseRootDeviceHintsIsValid{
			wwn:        "",
			raid:       Raid{},
			expectBool: false,
		}),
	)
})

var _ = Describe("Test RootDeviceHints.ListOfWWN", func() {
	type testCaseRootDeviceHintsListOfWWN struct {
		wwn        string
		raid       Raid
		expectList []string
	}

	DescribeTable("Test RootDeviceHints.ListOfWWN",
		func(tc testCaseRootDeviceHintsListOfWWN) {
			rdh := RootDeviceHints{
				WWN:  tc.wwn,
				Raid: tc.raid,
			}

			Expect(rdh.ListOfWWN()).Should(Equal(tc.expectList))
		},
		Entry("wwn set", testCaseRootDeviceHintsListOfWWN{
			wwn:        "test-wwn",
			raid:       Raid{},
			expectList: []string{"test-wwn"},
		}),
		Entry("raid set", testCaseRootDeviceHintsListOfWWN{
			wwn:        "",
			raid:       Raid{WWN: []string{"test-wwn1", "test-wwn2"}},
			expectList: []string{"test-wwn1", "test-wwn2"},
		}),
	)
})

var _ = Describe("Test HasSoftwareReboot", func() {
	type testCaseHasSoftwareReboot struct {
		rebootTypes []RebootType
		expectBool  bool
	}

	DescribeTable("Test HasSoftwareReboot",
		func(tc testCaseHasSoftwareReboot) {
			host := HetznerBareMetalHost{}
			host.Spec.Status.RebootTypes = tc.rebootTypes
			Expect(host.HasSoftwareReboot()).Should(Equal(tc.expectBool))
		},
		Entry("has software reboot - single reboot type", testCaseHasSoftwareReboot{
			rebootTypes: []RebootType{RebootTypeSoftware},
			expectBool:  true,
		}),
		Entry("has software reboot - multiple reboot types", testCaseHasSoftwareReboot{
			rebootTypes: []RebootType{RebootTypeHardware, RebootTypeSoftware},
			expectBool:  true,
		}),
		Entry("has no software reboot", testCaseHasSoftwareReboot{
			rebootTypes: []RebootType{RebootTypeHardware, RebootTypeManual},
			expectBool:  false,
		}),
	)
})

var _ = Describe("Test HasHardwareReboot", func() {
	type testCaseHasHardwareReboot struct {
		rebootTypes []RebootType
		expectBool  bool
	}

	DescribeTable("Test HasHardwareReboot",
		func(tc testCaseHasHardwareReboot) {
			host := HetznerBareMetalHost{}
			host.Spec.Status.RebootTypes = tc.rebootTypes
			Expect(host.HasHardwareReboot()).Should(Equal(tc.expectBool))
		},
		Entry("has hardware reboot - single reboot type", testCaseHasHardwareReboot{
			rebootTypes: []RebootType{RebootTypeHardware},
			expectBool:  true,
		}),
		Entry("has hardware reboot - multiple reboot types", testCaseHasHardwareReboot{
			rebootTypes: []RebootType{RebootTypeSoftware, RebootTypeHardware},
			expectBool:  true,
		}),
		Entry("has no hardware reboot", testCaseHasHardwareReboot{
			rebootTypes: []RebootType{RebootTypeSoftware, RebootTypeManual},
			expectBool:  false,
		}),
	)
})

var _ = Describe("Test HasPowerReboot", func() {
	type testCaseHasPowerReboot struct {
		rebootTypes []RebootType
		expectBool  bool
	}

	DescribeTable("Test HasPowerReboot",
		func(tc testCaseHasPowerReboot) {
			host := HetznerBareMetalHost{}
			host.Spec.Status.RebootTypes = tc.rebootTypes
			Expect(host.HasPowerReboot()).Should(Equal(tc.expectBool))
		},
		Entry("has power reboot - single reboot type", testCaseHasPowerReboot{
			rebootTypes: []RebootType{RebootTypePower},
			expectBool:  true,
		}),
		Entry("has power reboot - multiple reboot types", testCaseHasPowerReboot{
			rebootTypes: []RebootType{RebootTypeSoftware, RebootTypePower},
			expectBool:  true,
		}),
		Entry("has no power reboot", testCaseHasPowerReboot{
			rebootTypes: []RebootType{RebootTypeSoftware, RebootTypeManual},
			expectBool:  false,
		}),
	)
})

var _ = Describe("Test NeedsProvisioning", func() {
	type testCaseNeedsProvisioning struct {
		installImage *InstallImage
		expectBool   bool
	}

	DescribeTable("Test NeedsProvisioning",
		func(tc testCaseNeedsProvisioning) {
			host := HetznerBareMetalHost{}
			host.Spec.Status.InstallImage = tc.installImage
			Expect(host.NeedsProvisioning()).Should(Equal(tc.expectBool))
		},
		Entry("has installImage", testCaseNeedsProvisioning{
			installImage: &InstallImage{},
			expectBool:   true,
		}),
		Entry("has no installImage", testCaseNeedsProvisioning{
			installImage: nil,
			expectBool:   false,
		}),
	)
})

var _ = Describe("Test SetError", func() {
	type testCaseSetError struct {
		existingErrorCount   int
		existingErrorType    ErrorType
		existingErrorMessage string
		expectErrorCount     int
	}

	DescribeTable("Test SetError",
		func(tc testCaseSetError) {
			host := HetznerBareMetalHost{}
			host.Spec.Status.ErrorCount = tc.existingErrorCount
			host.Spec.Status.ErrorType = tc.existingErrorType
			host.Spec.Status.ErrorMessage = tc.existingErrorMessage

			errorType := ErrorType("test error type")
			errorMessage := "test error message"

			host.SetError(errorType, errorMessage)

			Expect(host.Spec.Status.ErrorCount).Should(Equal(tc.expectErrorCount))
			Expect(host.Spec.Status.ErrorType).Should(Equal(errorType))
			Expect(host.Spec.Status.ErrorMessage).Should(Equal(errorMessage))
		},
		Entry("no existing error", testCaseSetError{
			existingErrorCount:   0,
			existingErrorType:    ErrorType(""),
			existingErrorMessage: "",
			expectErrorCount:     1,
		}),
		Entry("existing error - different error", testCaseSetError{
			existingErrorCount:   2,
			existingErrorType:    ErrorTypeConnectionError,
			existingErrorMessage: "existing error message",
			expectErrorCount:     1,
		}),
		Entry("existing error - same error", testCaseSetError{
			existingErrorCount:   2,
			existingErrorType:    ErrorType("test error type"),
			existingErrorMessage: "test error message",
			expectErrorCount:     3,
		}),
	)
})

var _ = Describe("Test GetIPAddress", func() {
	type testCaseGetIPAddress struct {
		ipv4         string
		ipv6         string
		expectString string
	}

	DescribeTable("Test GetIPAddress",
		func(tc testCaseGetIPAddress) {
			status := ControllerGeneratedStatus{}
			status.IPv4 = tc.ipv4
			status.IPv6 = tc.ipv6

			Expect(status.GetIPAddress()).Should(Equal(tc.expectString))
		},
		Entry("ipv4 set", testCaseGetIPAddress{
			ipv4:         "127.0.0.1",
			ipv6:         "",
			expectString: "127.0.0.1",
		}),
		Entry("ipv6 set", testCaseGetIPAddress{
			ipv4:         "",
			ipv6:         "2001:db8:3333:4444:5555:6666:7777:8888",
			expectString: "2001:db8:3333:4444:5555:6666:7777:8888",
		}),
		Entry("ipv4 and ivp6 set", testCaseGetIPAddress{
			ipv4:         "127.0.0.1",
			ipv6:         "2001:db8:3333:4444:5555:6666:7777:8888",
			expectString: "127.0.0.1",
		}),
	)
})

var _ = Describe("Test ClearError", func() {
	type testCaseClearError struct {
		existingErrorCount   int
		existingErrorType    ErrorType
		existingErrorMessage string
	}

	DescribeTable("Test ClearError",
		func(tc testCaseClearError) {
			host := HetznerBareMetalHost{}
			host.Spec.Status.ErrorCount = tc.existingErrorCount
			host.Spec.Status.ErrorType = tc.existingErrorType
			host.Spec.Status.ErrorMessage = tc.existingErrorMessage

			host.ClearError()

			Expect(host.Spec.Status.ErrorCount).Should(Equal(0))
			Expect(host.Spec.Status.ErrorType).Should(Equal(ErrorType("")))
			Expect(host.Spec.Status.ErrorMessage).Should(Equal(""))
		},
		Entry("no existing error", testCaseClearError{
			existingErrorCount:   0,
			existingErrorType:    ErrorType(""),
			existingErrorMessage: "",
		}),
		Entry("existing error", testCaseClearError{
			existingErrorCount:   2,
			existingErrorType:    ErrorTypeConnectionError,
			existingErrorMessage: "existing error message",
		}),
	)
})

var _ = Describe("Test HasRebootAnnotation", func() {
	type testCaseHasRebootAnnotation struct {
		annotations map[string]string
		expectBool  bool
	}

	DescribeTable("Test HasRebootAnnotation",
		func(tc testCaseHasRebootAnnotation) {
			host := HetznerBareMetalHost{}
			host.SetAnnotations(tc.annotations)

			Expect(host.HasRebootAnnotation()).Should(Equal(tc.expectBool))
		},
		Entry("has reboot annotation - one annotation in list", testCaseHasRebootAnnotation{
			annotations: map[string]string{RebootAnnotation: "reboot"},
			expectBool:  true,
		}),
		Entry("has reboot annotation - multiple annotations in list", testCaseHasRebootAnnotation{
			annotations: map[string]string{"other": "annotation", RebootAnnotation: "reboot"},
			expectBool:  true,
		}),
		Entry("has no reboot annotation", testCaseHasRebootAnnotation{
			annotations: map[string]string{"other": "annotation", "another": "annotation"},
			expectBool:  false,
		}),
	)
})

var _ = Describe("Test ClearRebootAnnotations", func() {
	type testCaseClearRebootAnnotations struct {
		currentAnnotations map[string]string
		expectAnnotations  map[string]string
	}

	secondRebootAnnotation := RebootAnnotation + "/"

	DescribeTable("Test ClearRebootAnnotations",
		func(tc testCaseClearRebootAnnotations) {
			host := HetznerBareMetalHost{}
			host.SetAnnotations(tc.currentAnnotations)

			host.ClearRebootAnnotations()

			Expect(host.Annotations).Should(Equal(tc.expectAnnotations))
		},
		Entry("has reboot annotation - one annotation in list", testCaseClearRebootAnnotations{
			currentAnnotations: map[string]string{secondRebootAnnotation: "reboot", RebootAnnotation: "reboot"},
			expectAnnotations:  map[string]string{},
		}),
		Entry("has multiple reboot annotations - no other annotation in list", testCaseClearRebootAnnotations{
			currentAnnotations: map[string]string{RebootAnnotation: "reboot"},
			expectAnnotations:  map[string]string{},
		}),
		Entry("has reboot annotation - multiple annotations in list", testCaseClearRebootAnnotations{
			currentAnnotations: map[string]string{"other": "annotation", RebootAnnotation: "reboot"},
			expectAnnotations:  map[string]string{"other": "annotation"},
		}),
		Entry("has multiple reboot annotations - multiple annotations in list", testCaseClearRebootAnnotations{
			currentAnnotations: map[string]string{secondRebootAnnotation: "reboot", "other": "annotation", RebootAnnotation: "reboot"},
			expectAnnotations:  map[string]string{"other": "annotation"},
		}),
		Entry("has no reboot annotation", testCaseClearRebootAnnotations{
			currentAnnotations: map[string]string{"other": "annotation", "another": "annotation"},
			expectAnnotations:  map[string]string{"other": "annotation", "another": "annotation"},
		}),
	)
})

var _ = Describe("Test ClearRebootAnnotations", func() {
	type testCaseClearRebootAnnotations struct {
		annotation string
		expectBool bool
	}

	DescribeTable("Test ClearRebootAnnotations",
		func(tc testCaseClearRebootAnnotations) {
			Expect(isRebootAnnotation(tc.annotation)).Should(Equal(tc.expectBool))
		},
		Entry("reboot annotation", testCaseClearRebootAnnotations{
			annotation: RebootAnnotation,
			expectBool: true,
		}),
		Entry("reboot prefix", testCaseClearRebootAnnotations{
			annotation: RebootAnnotation + "/" + "suffix",
			expectBool: true,
		}),
		Entry("other annotation", testCaseClearRebootAnnotations{
			annotation: "different/annotation",
			expectBool: false,
		}),
	)
})
