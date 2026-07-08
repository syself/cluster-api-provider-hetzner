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

package remediation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
)

func TestBMRemediation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BMRemediation Suite")
}

var _ = Describe("Test TimeUntilNextRemediation", func() {
	type testCaseTimeUntilNextRemediation struct {
		lastRemediated                 time.Time
		expectTimeUntilLastRemediation time.Duration
	}

	now := time.Now()
	nullTime := time.Time{}

	DescribeTable("Test TimeUntilNextRemediation",
		func(tc testCaseTimeUntilNextRemediation) {
			var bmRemediation infrav1.HetznerBareMetalRemediation

			bmRemediation.Spec.Strategy = &infrav1.BareMetalRemediationStrategy{RemediationStrategy: infrav1.RemediationStrategy{Timeout: &metav1.Duration{Duration: time.Minute}}}

			if tc.lastRemediated != nullTime {
				bmRemediation.Status.LastRemediated = &metav1.Time{Time: tc.lastRemediated}
			}

			service := Service{scope: &scope.BareMetalRemediationScope{
				BareMetalRemediation: &bmRemediation,
			}}

			timeUntilNextRemediation := service.timeUntilNextRemediation(now)

			Expect(timeUntilNextRemediation).To(Equal(tc.expectTimeUntilLastRemediation))
		},
		Entry("first remediation", testCaseTimeUntilNextRemediation{
			lastRemediated:                 nullTime,
			expectTimeUntilLastRemediation: time.Minute,
		}),
		Entry("remediation timed out", testCaseTimeUntilNextRemediation{
			lastRemediated:                 now.Add(-2 * time.Minute),
			expectTimeUntilLastRemediation: time.Duration(0),
		}),
		Entry("remediation not timed out", testCaseTimeUntilNextRemediation{
			lastRemediated:                 now.Add(-30 * time.Second),
			expectTimeUntilLastRemediation: 31 * time.Second,
		}),
	)
})

var _ = Describe("Test AddRebootAnnotation", func() {
	type testCaseAddRebootAnnotation struct {
		annotations       map[string]string
		expectAnnotations map[string]string
	}

	rebootAnnotationArguments := infrav1.RebootAnnotationArguments{Type: infrav1.RebootTypeHardware}

	b, err := json.Marshal(rebootAnnotationArguments)
	Expect(err).To(BeNil())

	rebootAnnotationString := string(b)

	DescribeTable("Test AddRebootAnnotation",
		func(tc testCaseAddRebootAnnotation) {
			annotations, err := addRebootAnnotation(tc.annotations)

			Expect(annotations).To(Equal(tc.expectAnnotations))
			Expect(err).To(BeNil())
		},
		Entry("nil annotations", testCaseAddRebootAnnotation{
			annotations:       nil,
			expectAnnotations: map[string]string{infrav1.RebootAnnotation: rebootAnnotationString},
		}),
		Entry("existing annotations", testCaseAddRebootAnnotation{
			annotations:       map[string]string{"key": "value"},
			expectAnnotations: map[string]string{"key": "value", infrav1.RebootAnnotation: rebootAnnotationString},
		}),
		Entry("reboot annotation already present", testCaseAddRebootAnnotation{
			annotations:       map[string]string{"key": "value", infrav1.RebootAnnotation: rebootAnnotationString},
			expectAnnotations: map[string]string{"key": "value", infrav1.RebootAnnotation: rebootAnnotationString},
		}),
	)
})

var _ = Describe("Test handlePhaseWaiting onExhaustion", func() {
	// Deterministic unit test with a fake client (no host controller running, so
	// nothing overwrites the permanent error). It covers the case where reboots are
	// used up and the node is still unhealthy, so handlePhaseWaiting decides whether
	// to reuse or retire the host.
	scheme := runtime.NewScheme()
	utilruntime.Must(infrav1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))

	type testCaseOnExhaustion struct {
		onExhaustion             infrav1.OnExhaustionAction
		retryCount               int
		healthCheckMessage       string
		expectHostPermanentError bool
		expectErrorMessage       string
	}

	DescribeTable("retires the host only when onExhaustion is Retire",
		func(tc testCaseOnExhaustion) {
			ctx := context.Background()

			machine := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "test-machine", Namespace: "default", UID: "machine-uid"},
			}
			host := &infrav1.HetznerBareMetalHost{
				ObjectMeta: metav1.ObjectMeta{Name: "test-host", Namespace: "default"},
				Spec: infrav1.HetznerBareMetalHostSpec{
					Status: infrav1.ControllerGeneratedStatus{ProvisioningState: infrav1.StateProvisioned},
				},
			}
			remediation := &infrav1.HetznerBareMetalRemediation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-remediation",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "Machine",
						APIVersion: clusterv1.GroupVersion.String(),
						Name:       machine.Name,
						UID:        machine.UID,
					}},
				},
				Spec: infrav1.HetznerBareMetalRemediationSpec{
					Strategy: &infrav1.BareMetalRemediationStrategy{
						RemediationStrategy: infrav1.RemediationStrategy{
							Type:       infrav1.RemediationTypeReboot,
							RetryLimit: 1,
							Timeout:    &metav1.Duration{Duration: time.Second},
						},
						OnExhaustion: tc.onExhaustion,
					},
				},
				Status: infrav1.HetznerBareMetalRemediationStatus{
					Phase:          infrav1.PhaseWaiting,
					RetryCount:     tc.retryCount,
					LastRemediated: &metav1.Time{Time: time.Now().Add(-2 * time.Second)},
				},
			}

			// The MachineHealthCheck records the failing node condition on the Machine's
			// HealthCheckSucceeded condition. When present, retireHost uses it as the reason.
			if tc.healthCheckMessage != "" {
				conditions.Set(machine, metav1.Condition{
					Type:    clusterv1.MachineHealthCheckSucceededCondition,
					Status:  metav1.ConditionFalse,
					Reason:  clusterv1.MachineHealthCheckUnhealthyNodeReason,
					Message: tc.healthCheckMessage,
				})
			}

			c := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(host, machine, remediation).
				WithStatusSubresource(machine).
				Build()

			service := &Service{scope: &scope.BareMetalRemediationScope{
				Client:               c,
				Machine:              machine,
				BareMetalRemediation: remediation,
			}}

			// handlePhaseWaiting decides the node is healthy by checking the MachineNodeHealthy
			// condition on the owner Machine. This test machine has no such condition, so the
			// node is treated as unhealthy and the onExhaustion decision applies instead of
			// marking the remediation succeeded.
			res, err := service.handlePhaseWaiting(ctx, host)
			Expect(err).To(BeNil())
			Expect(res.RequeueAfter).To(BeZero())

			// Either way, remediation stops.
			Expect(remediation.Status.Phase).To(Equal(infrav1.PhaseDeleting))

			updatedHost := &infrav1.HetznerBareMetalHost{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(host), updatedHost)).To(Succeed())
			if tc.expectHostPermanentError {
				Expect(updatedHost.Spec.Status.ErrorType).To(Equal(infrav1.PermanentError))
				// We check the errorMessage against the expected one because its wording differs
				// for 0 reboots (retryLimit 0) versus one or more failed reboots.
				Expect(updatedHost.Spec.Status.ErrorMessage).To(Equal(tc.expectErrorMessage))
				Expect(updatedHost.Annotations).To(HaveKey(infrav1.PermanentErrorAnnotation))
			} else {
				Expect(updatedHost.Spec.Status.ErrorType).To(BeEmpty())
				Expect(updatedHost.Annotations).NotTo(HaveKey(infrav1.PermanentErrorAnnotation))
			}
		},
		Entry("Retire after failed reboots", testCaseOnExhaustion{
			onExhaustion:             infrav1.OnExhaustionRetire,
			retryCount:               1,
			expectHostPermanentError: true,
			expectErrorMessage:       "retired by remediation: node still unhealthy after 1 failed reboot(s)",
		}),
		Entry("Retire with no reboots (retryLimit 0)", testCaseOnExhaustion{
			onExhaustion:             infrav1.OnExhaustionRetire,
			retryCount:               0,
			expectHostPermanentError: true,
			expectErrorMessage:       "retired by remediation: retryLimit is 0, node retired without a reboot attempt",
		}),
		Entry("Reuse deletes the machine without retiring the host", testCaseOnExhaustion{
			onExhaustion:             infrav1.OnExhaustionReuse,
			retryCount:               1,
			expectHostPermanentError: false,
		}),
		Entry("empty behaves like Reuse", testCaseOnExhaustion{
			onExhaustion:             "",
			retryCount:               1,
			expectHostPermanentError: false,
		}),
		Entry("Retire uses the MachineHealthCheck reason when the condition is set", testCaseOnExhaustion{
			onExhaustion:             infrav1.OnExhaustionRetire,
			retryCount:               1,
			healthCheckMessage:       "Health check failed: Condition Ready on Node is reporting status Unknown for more than 5m0s",
			expectHostPermanentError: true,
			expectErrorMessage:       "retired by remediation: Health check failed: Condition Ready on Node is reporting status Unknown for more than 5m0s",
		}),
	)
})
