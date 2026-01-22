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

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

var _ = Describe("updateSSHKey", func() {
	type testCaseUpdateSSHKey struct {
		osSecretData             map[string][]byte
		rescueSecretData         map[string][]byte
		currentState             infrav1.ProvisioningState
		expectedActionResult     actionResult
		expectedNextState        infrav1.ProvisioningState
		expectedOSSecretData     map[string][]byte
		expectedRescueSecretData map[string][]byte
	}

	DescribeTable("updateSSHKey",
		func(tc testCaseUpdateSSHKey) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHStatus(),
				helpers.WithSSHSpecInclPorts(23),
			)

			dataHashOS, err := infrav1.HashOfSecretData(tc.osSecretData)
			Expect(err).To(BeNil())

			dataHashRescue, err := infrav1.HashOfSecretData(tc.rescueSecretData)
			Expect(err).To(BeNil())

			expectedDataHashOS, err := infrav1.HashOfSecretData(tc.expectedOSSecretData)
			Expect(err).To(BeNil())

			expectedDataHashRescue, err := infrav1.HashOfSecretData(tc.expectedRescueSecretData)
			Expect(err).To(BeNil())

			host.Spec.Status.SSHStatus.CurrentOS = &infrav1.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      osSSHKeyName,
					Namespace: "default",
				},
				DataHash: dataHashOS,
			}
			host.Spec.Status.SSHStatus.CurrentRescue = &infrav1.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      rescueSSHKeyName,
					Namespace: "default",
				},
				DataHash: dataHashRescue,
			}
			host.Spec.Status.ProvisioningState = tc.currentState

			osSSHSecret := helpers.GetDefaultSSHSecret(osSSHKeyName, "default")
			osSSHSecret.ObjectMeta.ResourceVersion = "1"

			rescueSSHSecret := helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default")
			rescueSSHSecret.ObjectMeta.ResourceVersion = "1"

			service := newTestService(host, nil, nil, osSSHSecret, rescueSSHSecret)
			hsm := newTestHostStateMachine(host, service)

			actResult := hsm.updateSSHKey()

			Expect(actResult).Should(BeAssignableToTypeOf(tc.expectedActionResult))
			Expect(*host.Spec.Status.SSHStatus.CurrentRescue).Should(Equal(infrav1.SecretStatus{
				Reference: &corev1.SecretReference{
					Name:      rescueSSHKeyName,
					Namespace: "default",
				},
				DataHash: expectedDataHashRescue,
			}))
			Expect(*host.Spec.Status.SSHStatus.CurrentOS).Should(Equal(infrav1.SecretStatus{
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
			currentState:         infrav1.StateRegistering,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav1.StateRegistering,
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
			currentState:         infrav1.StateRegistering,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav1.StateRegistering,
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
			currentState:         infrav1.StateProvisioned,
			expectedActionResult: actionFailed{},
			expectedNextState:    infrav1.StateProvisioned,
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
			currentState:         infrav1.StateImageInstalling,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav1.StateImageInstalling,
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
			currentState:         infrav1.StateRegistering,
			expectedActionResult: actionComplete{},
			expectedNextState:    infrav1.StateNone,
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
