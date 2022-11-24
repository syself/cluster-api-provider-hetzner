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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("updateSSHKey", func() {
	DescribeTable("updateSSHKey",
		func(
			osSecretData map[string][]byte,
			rescueSecretData map[string][]byte,
			currentState infrav1.ProvisioningState,
			expectedActionResult actionResult,
			expectedNextState infrav1.ProvisioningState,
			expectedOSSecretData map[string][]byte,
			expectedRescueSecretData map[string][]byte,
		) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHStatus(),
				helpers.WithSSHSpecInclPorts(23, 24),
			)

			dataHashOS, err := infrav1.HashOfSecretData(osSecretData)
			Expect(err).To(BeNil())

			dataHashRescue, err := infrav1.HashOfSecretData(rescueSecretData)
			Expect(err).To(BeNil())

			expectedDataHashOS, err := infrav1.HashOfSecretData(expectedOSSecretData)
			Expect(err).To(BeNil())

			expectedDataHashRescue, err := infrav1.HashOfSecretData(expectedRescueSecretData)
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
			host.Spec.Status.ProvisioningState = currentState

			osSSHSecret := helpers.GetDefaultSSHSecret(osSSHKeyName, "default")
			osSSHSecret.ObjectMeta.ResourceVersion = "1"

			rescueSSHSecret := helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default")
			rescueSSHSecret.ObjectMeta.ResourceVersion = "1"

			service := newTestService(host, nil, nil, osSSHSecret, rescueSSHSecret)
			hsm := newTestHostStateMachine(host, service)

			actResult := hsm.updateSSHKey()

			Expect(actResult).Should(BeAssignableToTypeOf(expectedActionResult))
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
			Expect(hsm.nextState).Should(Equal(expectedNextState))
		},
		Entry(
			"nothing changed",
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // osSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // rescueSecretData string
			infrav1.StateRegistering, // currentState infrav1.ProvisioningState
			actionComplete{},         // expectedActionResult actionResult
			infrav1.StateRegistering, // expectedNextState infrav1.ProvisioningState
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedOSSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedRescueSecretData string
		),
		Entry(
			"os secret changed - state available",
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			}, // osSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // rescueSecretData string
			infrav1.StateRegistering, // currentState infrav1.ProvisioningState
			actionComplete{},         // expectedActionResult actionResult
			infrav1.StateRegistering, // expectedNextState infrav1.ProvisioningState
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedOSSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedRescueSecretData string
		),
		Entry(
			"os secret changed - state provisioned",
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			}, // osSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // rescueSecretData string
			infrav1.StateProvisioned, // currentState infrav1.ProvisioningState
			actionFailed{},           // expectedActionResult actionResult
			infrav1.StateProvisioned, // expectedNextState infrav1.ProvisioningState
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			}, // expectedOSSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedRescueSecretData string
		),
		Entry(
			"os secret changed - state provisioning",
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			}, // osSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // rescueSecretData string
			infrav1.StateProvisioning,    // currentState infrav1.ProvisioningState
			actionComplete{},             // expectedActionResult actionResult
			infrav1.StateImageInstalling, // expectedNextState infrav1.ProvisioningState
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedOSSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedRescueSecretData string
		),
		Entry(
			"rescue secret changed - state provisioning",
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // osSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			}, // rescueSecretData string
			infrav1.StateProvisioning, // currentState infrav1.ProvisioningState
			actionComplete{},          // expectedActionResult actionResult
			infrav1.StateProvisioning, // expectedNextState infrav1.ProvisioningState
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedOSSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedRescueSecretData string
		),
		Entry(
			"rescue secret changed - state available",
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // osSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-old-name"),
				"public-key":  []byte("my-old-public-key"),
			}, // rescueSecretData string
			infrav1.StateRegistering, // currentState infrav1.ProvisioningState
			actionComplete{},         // expectedActionResult actionResult
			infrav1.StateNone,        // expectedNextState infrav1.ProvisioningState
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", osSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedOSSecretData string
			map[string][]byte{
				"private-key": []byte(fmt.Sprintf("%s-private-key", rescueSSHKeyName)),
				"sshkey-name": []byte("my-name"),
				"public-key":  []byte("my-public-key"),
			}, // expectedRescueSecretData string
		),
	)
})
