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

// HCloudRemediation has no Set...Summary helper in this package, so these tests call
// NewSummaryCondition the same way HCloudRemediationScope.Close does.
var _ = Describe("HCloudRemediationSummaryOpts", func() {
	It("reports Ready=Unknown when no conditions are set yet", func() {
		hcloudRemediation := &infrav2.HCloudRemediation{}

		ready, err := conditions.NewSummaryCondition(
			hcloudRemediation,
			clusterv1.ReadyCondition,
			infrav2.HCloudRemediationSummaryOpts()...,
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionUnknown))
		Expect(ready.Reason).To(Equal(clusterv1.ReadyUnknownReason))
	})

	It("reports Ready=True when the token is available and the negative polarity conditions are absent", func() {
		hcloudRemediation := &infrav2.HCloudRemediation{}

		hcloudRemediation.SetConditions([]metav1.Condition{
			{
				Type:   infrav2.HCloudTokenAvailableCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HCloudTokenAvailableReason,
			},
		})

		ready, err := conditions.NewSummaryCondition(
			hcloudRemediation,
			clusterv1.ReadyCondition,
			infrav2.HCloudRemediationSummaryOpts()...,
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(clusterv1.ReadyReason))
	})
})
