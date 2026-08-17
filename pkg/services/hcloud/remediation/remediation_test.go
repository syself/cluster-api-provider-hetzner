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

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	"github.com/syself/cluster-api-provider-hetzner/pkg/scope"
	"github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/mocks"
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

var _ = Describe("Test rate limit condition", func() {
	It("sets HetznerAPIReachable to false on the HCloudRemediation when a reboot hits the rate limit", func() {
		hcloudRemediation := &infrav1.HCloudRemediation{
			ObjectMeta: metav1.ObjectMeta{Name: "my-remediation", Namespace: "default"},
			Spec: infrav1.HCloudRemediationSpec{
				Strategy: &infrav1.RemediationStrategy{
					Type:       infrav1.RemediationTypeReboot,
					RetryLimit: 1,
					Timeout:    &metav1.Duration{Duration: time.Minute},
				},
			},
			Status: infrav1.HCloudRemediationStatus{Phase: infrav1.PhaseRunning},
		}

		hcloudClient := mocks.NewClient(GinkgoT())
		rateLimitErr := hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded, Message: "rate limit exceeded"}
		hcloudClient.On("RebootServer", mock.Anything, mock.Anything).Return(rateLimitErr).Once()

		service := Service{scope: &scope.HCloudRemediationScope{
			HCloudClient:      hcloudClient,
			HCloudRemediation: hcloudRemediation,
		}}

		_, err := service.handlePhaseRunning(context.Background(), &hcloud.Server{ID: 1234567, Name: "my-server"})
		Expect(err).To(HaveOccurred())
		Expect(hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded)).To(BeTrue())

		// the controller reads HetznerAPIReachable on the HCloudRemediation to back off while rate limited.
		Expect(v1beta1conditions.IsFalse(hcloudRemediation, infrav1.HetznerAPIReachableCondition)).To(BeTrue())
	})
})
