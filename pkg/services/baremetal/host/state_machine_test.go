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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

var _ = Describe("updateSSHKey", func() {
	type testCaseUpdateSSHKey struct {
		osSecretData             map[string][]byte
		rescueSecretData         map[string][]byte
		currentState             infrav2.ProvisioningState
		expectedActionResult     actionResult
		expectedNextState        infrav2.ProvisioningState
		expectedOSSecretData     map[string][]byte
		expectedRescueSecretData map[string][]byte
	}

	DescribeTable("updateSSHKey",
		func(tc testCaseUpdateSSHKey) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHStatus(),
			)

			dataHashOS, err := infrav2.HashOfSecretData(tc.osSecretData)
			Expect(err).To(BeNil())

			dataHashRescue, err := infrav2.HashOfSecretData(tc.rescueSecretData)
			Expect(err).To(BeNil())

			expectedDataHashOS, err := infrav2.HashOfSecretData(tc.expectedOSSecretData)
			Expect(err).To(BeNil())

			expectedDataHashRescue, err := infrav2.HashOfSecretData(tc.expectedRescueSecretData)
			Expect(err).To(BeNil())

			host.Status.SSHStatus.CurrentOS = &infrav2.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      osSSHKeyName,
					Namespace: "default",
				},
				DataHash: dataHashOS,
			}
			host.Status.SSHStatus.CurrentRescue = &infrav2.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      rescueSSHKeyName,
					Namespace: "default",
				},
				DataHash: dataHashRescue,
			}
			host.Status.ProvisioningState = tc.currentState

			osSSHSecret := helpers.GetDefaultSSHSecret(osSSHKeyName, "default")
			osSSHSecret.ResourceVersion = "1"

			rescueSSHSecret := helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default")
			rescueSSHSecret.ResourceVersion = "1"

			service := newTestService(host, nil, nil, osSSHSecret, rescueSSHSecret)
			hsm := newTestHostStateMachine(host, service)

			actResult := hsm.updateSSHKey()

			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
			Expect(*host.Status.SSHStatus.CurrentRescue).Should(Equal(infrav2.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      rescueSSHKeyName,
					Namespace: "default",
				},
				DataHash: expectedDataHashRescue,
			}))
			Expect(*host.Status.SSHStatus.CurrentOS).Should(Equal(infrav2.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      osSSHKeyName,
					Namespace: "default",
				},
				DataHash: expectedDataHashOS,
			}))
			Expect(hsm.nextState).Should(Equal(tc.expectedNextState))
		},
		Entry("nothing changed", testCaseUpdateSSHKey{
			osSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			rescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			currentState:         infrav2.StateRegistering,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav2.StateRegistering,
			expectedOSSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			expectedRescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
		}),
		Entry("os secret changed - state available", testCaseUpdateSSHKey{
			osSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			},
			rescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			currentState:         infrav2.StateRegistering,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav2.StateRegistering,
			expectedOSSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			expectedRescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
		}),
		Entry("os secret changed - state provisioned", testCaseUpdateSSHKey{
			osSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			},
			rescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			currentState:         infrav2.StateProvisioned,
			expectedActionResult: actionContinue{},
			expectedNextState:    infrav2.StateProvisioned,
			expectedOSSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			},
			expectedRescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
		}),
		Entry("os secret changed - state provisioning", testCaseUpdateSSHKey{
			osSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			},
			rescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			currentState:         infrav2.StateImageInstalling,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav2.StateImageInstalling,
			expectedOSSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			expectedRescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
		}),
		Entry("rescue secret changed - state available", testCaseUpdateSSHKey{
			osSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			rescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			},
			currentState:         infrav2.StateRegistering,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav2.StateNone,
			expectedOSSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
			expectedRescueSecretData: map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			},
		}),
	)
})

var _ = Describe("provisioningCancelled", func() {
	It("returns false when the consuming machine exists and is not being deleted", func() {
		host := helpers.BareMetalHost("test-host", "default")
		service := newTestService(host, nil, nil, nil, nil)
		hsm := newTestHostStateMachine(host, service)

		Expect(hsm.provisioningCancelled()).To(BeFalse())
	})

	It("returns true when the consuming machine is being deleted", func() {
		host := helpers.BareMetalHost("test-host", "default")
		service := newTestService(host, nil, nil, nil, nil)
		hsm := newTestHostStateMachine(host, service)

		now := metav1.Now()
		service.scope.HetznerBareMetalMachine.DeletionTimestamp = &now

		Expect(hsm.provisioningCancelled()).To(BeTrue())
	})

	It("returns true when the consuming machine is gone", func() {
		host := helpers.BareMetalHost("test-host", "default")
		service := newTestService(host, nil, nil, nil, nil)
		hsm := newTestHostStateMachine(host, service)

		service.scope.HetznerBareMetalMachine = nil

		Expect(hsm.provisioningCancelled()).To(BeTrue())
	})

	It("returns true when the owner CAPI machine is being deleted", func() {
		host := helpers.BareMetalHost("test-host", "default")
		service := newTestService(host, nil, nil, nil, nil)
		hsm := newTestHostStateMachine(host, service)

		now := metav1.Now()
		service.scope.Machine.DeletionTimestamp = &now

		Expect(hsm.provisioningCancelled()).To(BeTrue())
	})

	It("returns true when the owner CAPI machine is gone", func() {
		host := helpers.BareMetalHost("test-host", "default")
		service := newTestService(host, nil, nil, nil, nil)
		hsm := newTestHostStateMachine(host, service)

		service.scope.Machine = nil

		Expect(hsm.provisioningCancelled()).To(BeTrue())
	})
})
