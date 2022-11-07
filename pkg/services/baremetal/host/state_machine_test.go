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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2/extensions/table"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("updateSSHKey", func() {
	DescribeTable("updateSSHKey",
		func(
			osSecretVersion string,
			rescueSecretVersion string,
			currentState infrav1.ProvisioningState,
			expectedActionResult actionResult,
			expectedNextState infrav1.ProvisioningState,
			expectedOSSecretVersion string,
			expectedRescueSecretVersion string,
		) {
			host := helpers.BareMetalHost(
				"test-host",
				"default",
				helpers.WithSSHStatus(),
				helpers.WithSSHSpecInclPorts(23, 24),
			)
			host.Spec.Status.SSHStatus.CurrentOS = &infrav1.SecretStatus{
				Version: osSecretVersion,
				Reference: &corev1.SecretReference{
					Name:      osSSHKeyName,
					Namespace: "default",
				},
			}
			host.Spec.Status.SSHStatus.CurrentRescue = &infrav1.SecretStatus{
				Version: rescueSecretVersion,
				Reference: &corev1.SecretReference{
					Name:      rescueSSHKeyName,
					Namespace: "default",
				},
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
				Version: expectedRescueSecretVersion,
				Reference: &corev1.SecretReference{
					Name:      rescueSSHKeyName,
					Namespace: "default",
				},
			}))
			Expect(*host.Spec.Status.SSHStatus.CurrentOS).Should(Equal(infrav1.SecretStatus{
				Version: expectedOSSecretVersion,
				Reference: &corev1.SecretReference{
					Name:      osSSHKeyName,
					Namespace: "default",
				},
			}))
			Expect(hsm.nextState).Should(Equal(expectedNextState))
		},
		Entry(
			"nothing changed",
			"1",                      // osSecretVersion string
			"1",                      // rescueSecretVersion string
			infrav1.StateRegistering, // currentState infrav1.ProvisioningState
			actionComplete{},         // expectedActionResult actionResult
			infrav1.StateRegistering, // expectedNextState infrav1.ProvisioningState
			"1",                      // expectedOSSecretVersion string
			"1",                      // expectedRescueSecretVersion string
		),
		Entry(
			"os secret changed - state available",
			"0",                      // osSecretVersion string
			"1",                      // rescueSecretVersion string
			infrav1.StateRegistering, // currentState infrav1.ProvisioningState
			actionComplete{},         // expectedActionResult actionResult
			infrav1.StateRegistering, // expectedNextState infrav1.ProvisioningState
			"1",                      // expectedOSSecretVersion string
			"1",                      // expectedRescueSecretVersion string
		),
		Entry(
			"os secret changed - state provisioned",
			"0",                      // osSecretVersion string
			"1",                      // rescueSecretVersion string
			infrav1.StateProvisioned, // currentState infrav1.ProvisioningState
			actionFailed{},           // expectedActionResult actionResult
			infrav1.StateProvisioned, // expectedNextState infrav1.ProvisioningState
			"0",                      // expectedOSSecretVersion string
			"1",                      // expectedRescueSecretVersion string
		),
		Entry(
			"os secret changed - state provisioning",
			"0",                          // osSecretVersion string
			"1",                          // rescueSecretVersion string
			infrav1.StateProvisioning,    // currentState infrav1.ProvisioningState
			actionComplete{},             // expectedActionResult actionResult
			infrav1.StateImageInstalling, // expectedNextState infrav1.ProvisioningState
			"1",                          // expectedOSSecretVersion string
			"1",                          // expectedRescueSecretVersion string
		),
		Entry(
			"rescue secret changed - state provisioning",
			"1",                       // osSecretVersion string
			"0",                       // rescueSecretVersion string
			infrav1.StateProvisioning, // currentState infrav1.ProvisioningState
			actionComplete{},          // expectedActionResult actionResult
			infrav1.StateProvisioning, // expectedNextState infrav1.ProvisioningState
			"1",                       // expectedOSSecretVersion string
			"1",                       // expectedRescueSecretVersion string
		),
		Entry(
			"rescue secret changed - state available",
			"1",                      // osSecretVersion string
			"0",                      // rescueSecretVersion string
			infrav1.StateRegistering, // currentState infrav1.ProvisioningState
			actionComplete{},         // expectedActionResult actionResult
			infrav1.StateNone,        // expectedNextState infrav1.ProvisioningState
			"1",                      // expectedOSSecretVersion string
			"1",                      // expectedRescueSecretVersion string
		),
	)
})
