/*
Copyright 2026 The Kubernetes Authors.

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

package scope

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

var _ = Describe("HCloudMachineTemplateSummaryOpts", func() {
	It("returns an Unknown summary when the object has no conditions", func() {
		hcloudMachineTemplate := &infrav2.HCloudMachineTemplate{}

		readyCondition, err := conditions.NewSummaryCondition(hcloudMachineTemplate, clusterv1.ReadyCondition, infrav2.HCloudMachineTemplateSummaryOpts()...)
		Expect(err).To(BeNil())
		Expect(readyCondition).ToNot(BeNil())
		Expect(readyCondition.Status).To(Equal(metav1.ConditionUnknown))
	})

	It("lists all unhealthy conditions in priority order in the summary message", func() {
		hcloudMachineTemplate := &infrav2.HCloudMachineTemplate{}

		hcloudMachineTemplate.SetConditions([]metav1.Condition{
			// Available=False (lowest priority issue).
			{
				Type:    infrav2.HCloudMachineTemplateAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudMachineTemplateServerTypeNotFoundReason,
				Message: "server type is not offered",
			},
			// HCloudTokenAvailable=False (highest priority issue).
			{
				Type:    infrav2.HCloudTokenAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudTokenInvalidReason,
				Message: "token is invalid",
			},
		})

		readyCondition, err := conditions.NewSummaryCondition(hcloudMachineTemplate, clusterv1.ReadyCondition, infrav2.HCloudMachineTemplateSummaryOpts()...)
		Expect(err).To(BeNil())
		Expect(readyCondition).ToNot(BeNil())
		Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))

		// The summary message lists all unhealthy conditions in ForConditionTypes order.
		// HCloudTokenAvailable before Available.
		Expect(readyCondition.Message).To(MatchRegexp(`(?s)token is invalid.*server type is not offered`))
	})

	It("surfaces HCloudRateLimitExceeded before Available when both are unhealthy", func() {
		hcloudMachineTemplate := &infrav2.HCloudMachineTemplate{}

		hcloudMachineTemplate.SetConditions([]metav1.Condition{
			// HCloudRateLimitExceeded=True (negative polarity).
			{
				Type:    infrav2.HCloudRateLimitExceededCondition,
				Status:  metav1.ConditionTrue,
				Reason:  infrav2.HCloudRateLimitExceededReason,
				Message: "rate limit exceeded",
			},
			// Available=False with ServerTypeNotFound reason.
			{
				Type:    infrav2.HCloudMachineTemplateAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudMachineTemplateServerTypeNotFoundReason,
				Message: "server type is not offered",
			},
		})

		readyCondition, err := conditions.NewSummaryCondition(hcloudMachineTemplate, clusterv1.ReadyCondition, infrav2.HCloudMachineTemplateSummaryOpts()...)
		Expect(err).To(BeNil())
		Expect(readyCondition).ToNot(BeNil())

		// HCloudRateLimitExceeded before Available.
		Expect(readyCondition.Message).To(MatchRegexp(`(?s)rate limit exceeded.*server type is not offered`))
	})
})
