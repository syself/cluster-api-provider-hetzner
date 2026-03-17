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
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	hcloudfake "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

func TestHCloudRemediation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HCloudRemediation Suite")
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
			var bmRemediation infrav1.HCloudRemediation

			bmRemediation.Spec.Strategy = &infrav1.RemediationStrategy{Timeout: &metav1.Duration{Duration: time.Minute}}

			if tc.lastRemediated != nullTime {
				bmRemediation.Status.LastRemediated = &metav1.Time{Time: tc.lastRemediated}
			}

			service := Service{scope: &scope.HCloudRemediationScope{
				HCloudRemediation: &bmRemediation,
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

var _ = Describe("Test Reconcile", func() {
	It("treats a missing providerID as a missing server and proceeds to deleting", func() {
		scheme := runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(infrav1.AddToScheme(scheme)).To(Succeed())

		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: "default",
			},
		}
		hcloudMachine := &infrav1.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hcloud-machine",
				Namespace: "default",
			},
		}
		hcloudRemediation := &infrav1.HCloudRemediation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "remediation",
				Namespace: "default",
			},
			Spec: infrav1.HCloudRemediationSpec{
				Strategy: &infrav1.RemediationStrategy{
					Type:    infrav1.RemediationTypeReboot,
					Timeout: &metav1.Duration{Duration: time.Minute},
				},
			},
		}

		c := fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(machine, hcloudRemediation).
			WithObjects(machine.DeepCopy(), hcloudMachine.DeepCopy(), hcloudRemediation.DeepCopy()).
			Build()

		remediationScope, err := scope.NewHCloudRemediationScope(scope.HCloudRemediationScopeParams{
			Client:            c,
			Logger:            funcr.New(func(string, string) {}, funcr.Options{}),
			Machine:           machine,
			HCloudMachine:     hcloudMachine,
			HCloudRemediation: hcloudRemediation,
			HCloudClient:      hcloudfake.NewHCloudClientFactory().NewClient(""),
		})
		Expect(err).NotTo(HaveOccurred())

		result, err := NewService(remediationScope).Reconcile(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).To(BeZero())
		Expect(hcloudRemediation.Status.Phase).To(Equal(infrav1.PhaseDeleting))

		updatedMachine := &clusterv1.Machine{}
		Expect(c.Get(context.Background(), client.ObjectKeyFromObject(machine), updatedMachine)).To(Succeed())

		condition := conditions.Get(updatedMachine, clusterv1.MachineOwnerRemediatedCondition)
		Expect(condition).NotTo(BeNil())
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		Expect(condition.Reason).To(Equal(clusterv1.WaitingForRemediationReason))
	})
})
