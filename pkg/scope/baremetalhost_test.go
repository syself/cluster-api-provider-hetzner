/*
Copyright 2024 The Kubernetes Authors.

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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("SetHetznerBareMetalHostV1Beta2ReadySummary", func() {
	It("sets Ready=False with reason NotReady when ActionCompleted=False (PermanentError)", func() {
		host := &infrav1.HetznerBareMetalHost{}

		v1beta2conditions.Set(host, metav1.Condition{
			Type:   infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason,
		})
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostActionCompletedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostActionCompletedPermanentErrorV1Beta2Reason,
			Message: "reboot to rescue mode timed out",
		})

		SetHetznerBareMetalHostV1Beta2ReadySummary(host)

		ready := v1beta2conditions.Get(host, clusterv1beta1.ReadyV1Beta2Condition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1beta1.NotReadyV1Beta2Reason))
		Expect(ready.Message).To(ContainSubstring("reboot to rescue mode timed out"))
	})

	It("ActionCompleted=False takes priority over other non-credential conditions", func() {
		host := &infrav1.HetznerBareMetalHost{}

		v1beta2conditions.Set(host, metav1.Condition{
			Type:   infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Reason,
		})
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostActionCompletedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostActionCompletedPermanentErrorV1Beta2Reason,
			Message: "reboot to rescue mode timed out",
		})
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostProvisionSucceededV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostServerNotFoundV1Beta2Reason,
			Message: "server not found",
		})

		SetHetznerBareMetalHostV1Beta2ReadySummary(host)

		ready := v1beta2conditions.Get(host, clusterv1beta1.ReadyV1Beta2Condition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1beta1.NotReadyV1Beta2Reason))
		// The summary lists all failing conditions in priority order. ActionCompleted (priority 2)
		// must appear before ProvisionSucceeded (priority 6).
		Expect(ready.Message).To(ContainSubstring("reboot to rescue mode timed out"))
		Expect(ready.Message).To(ContainSubstring("server not found"))
		Expect(strings.Index(ready.Message, "reboot to rescue mode timed out")).
			To(BeNumerically("<", strings.Index(ready.Message, "server not found")))
	})

	It("RobotCredentialsAvailable=False takes priority over ActionCompleted=False", func() {
		host := &infrav1.HetznerBareMetalHost{}

		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostRobotCredentialsAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostRobotCredentialsInvalidV1Beta2Reason,
			Message: "invalid credentials",
		})
		v1beta2conditions.Set(host, metav1.Condition{
			Type:    infrav1.HetznerBareMetalHostActionCompletedV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.HetznerBareMetalHostActionCompletedPermanentErrorV1Beta2Reason,
			Message: "reboot to rescue mode timed out",
		})

		SetHetznerBareMetalHostV1Beta2ReadySummary(host)

		ready := v1beta2conditions.Get(host, clusterv1beta1.ReadyV1Beta2Condition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1beta1.NotReadyV1Beta2Reason))
		// The summary lists all failing conditions in priority order. RobotCredentialsAvailable
		// (priority 1) must appear before ActionCompleted (priority 2).
		Expect(ready.Message).To(ContainSubstring("invalid credentials"))
		Expect(ready.Message).To(ContainSubstring("reboot to rescue mode timed out"))
		Expect(strings.Index(ready.Message, "invalid credentials")).
			To(BeNumerically("<", strings.Index(ready.Message, "reboot to rescue mode timed out")))
	})
})
