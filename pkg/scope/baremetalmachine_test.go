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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("SetHetznerBareMetalMachineV1Beta2ReadySummary", func() {
	It("reports Ready=Unknown when no conditions are set yet", func() {
		hbmm := &infrav1.HetznerBareMetalMachine{}

		SetHetznerBareMetalMachineV1Beta2ReadySummary(hbmm)

		ready := v1beta2conditions.Get(hbmm, clusterv1beta1.ReadyV1Beta2Condition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionUnknown))
		Expect(ready.Reason).To(Equal(clusterv1beta1.ReadyUnknownV1Beta2Reason))
	})

	It("reports Ready=True once all required conditions are True", func() {
		hbmm := &infrav1.HetznerBareMetalMachine{}

		for _, c := range []metav1.Condition{
			{
				Type:   infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HCloudTokenAvailableV1Beta2Reason,
			},
			{
				Type:   infrav1.HetznerBareMetalMachineHostAssociatedV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HetznerBareMetalMachineHostAssociatedV1Beta2Reason,
			},
			{
				Type:   infrav1.HetznerBareMetalMachineHostReadyV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HetznerBareMetalMachineHostReadyV1Beta2Reason,
			},
			{
				Type:   infrav1.HetznerBareMetalMachineServerAvailableV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: infrav1.HetznerBareMetalMachineServerAvailableV1Beta2Reason,
			},
		} {
			v1beta2conditions.Set(hbmm, c)
		}

		SetHetznerBareMetalMachineV1Beta2ReadySummary(hbmm)

		ready := v1beta2conditions.Get(hbmm, clusterv1beta1.ReadyV1Beta2Condition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(clusterv1beta1.ReadyV1Beta2Reason))
	})
})
