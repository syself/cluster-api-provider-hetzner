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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

var _ = Describe("SetHetznerBareMetalMachineReadySummary", func() {
	It("reports Ready=Unknown when no conditions are set yet", func() {
		hbmm := &infrav2.HetznerBareMetalMachine{}

		SetHetznerBareMetalMachineReadySummary(hbmm)

		ready := conditions.Get(hbmm, clusterv1.ReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionUnknown))
		Expect(ready.Reason).To(Equal(clusterv1.ReadyUnknownReason))
	})

	It("reports Ready=True once all required conditions are True", func() {
		hbmm := &infrav2.HetznerBareMetalMachine{}

		for _, c := range []metav1.Condition{
			{
				Type:   infrav2.HCloudTokenAvailableCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HCloudTokenAvailableReason,
			},
			{
				Type:   infrav2.HetznerBareMetalMachineHostAssociatedCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HetznerBareMetalMachineHostAssociatedReason,
			},
			{
				Type:   infrav2.HetznerBareMetalMachineHostReadyCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HetznerBareMetalMachineHostReadyReason,
			},
			{
				Type:   infrav2.HetznerBareMetalMachineServerAvailableCondition,
				Status: metav1.ConditionTrue,
				Reason: infrav2.HetznerBareMetalMachineServerAvailableReason,
			},
		} {
			conditions.Set(hbmm, c)
		}

		SetHetznerBareMetalMachineReadySummary(hbmm)

		ready := conditions.Get(hbmm, clusterv1.ReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(clusterv1.ReadyReason))
	})

	It("sets Ready=False with reason NotReady when a summary condition is False", func() {
		hbmm := &infrav2.HetznerBareMetalMachine{}

		conditions.Set(hbmm, metav1.Condition{
			Type:   infrav2.HCloudTokenAvailableCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HCloudTokenAvailableReason,
		})
		conditions.Set(hbmm, metav1.Condition{
			Type:    infrav2.HetznerBareMetalMachineHostReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalMachineHostNotReadyReason,
			Message: "host is not ready",
		})

		SetHetznerBareMetalMachineReadySummary(hbmm)

		ready := conditions.Get(hbmm, clusterv1.ReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1.NotReadyReason))
		Expect(ready.Message).To(ContainSubstring("host is not ready"))
	})

	It("lists Deleting ahead of a HostReady failure", func() {
		hbmm := &infrav2.HetznerBareMetalMachine{}

		conditions.Set(hbmm, metav1.Condition{
			Type:   infrav2.HCloudTokenAvailableCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HCloudTokenAvailableReason,
		})
		conditions.Set(hbmm, metav1.Condition{
			Type:   infrav2.HetznerBareMetalMachineHostAssociatedCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HetznerBareMetalMachineHostAssociatedReason,
		})
		conditions.Set(hbmm, metav1.Condition{
			Type:    infrav2.HetznerBareMetalMachineHostReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalMachineHostNotReadyReason,
			Message: "host is not ready",
		})
		conditions.Set(hbmm, metav1.Condition{
			Type:    infrav2.HetznerBareMetalMachineDeletingCondition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav2.HetznerBareMetalMachineDeletingReason,
			Message: "waiting for host to deprovision",
		})

		SetHetznerBareMetalMachineReadySummary(hbmm)

		ready := conditions.Get(hbmm, clusterv1.ReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1.NotReadyReason))
		Expect(ready.Message).To(ContainSubstring("waiting for host to deprovision"))
		Expect(ready.Message).To(ContainSubstring("host is not ready"))
		Expect(strings.Index(ready.Message, "waiting for host to deprovision")).
			To(BeNumerically("<", strings.Index(ready.Message, "host is not ready")))
	})
})
