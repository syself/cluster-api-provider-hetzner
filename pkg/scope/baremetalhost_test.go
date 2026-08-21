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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

var _ = Describe("SetHetznerBareMetalHostReadySummary", func() {
	It("sets Ready=False with reason NotReady when a summary condition is False", func() {
		host := &infrav2.HetznerBareMetalHost{}

		conditions.Set(host, metav1.Condition{
			Type:   infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav2.HetznerBareMetalHostRobotCredentialsAvailableReason,
		})
		conditions.Set(host, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostProvisionSucceededCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostServerNotFoundReason,
			Message: "server not found",
		})

		SetHetznerBareMetalHostReadySummary(host)

		ready := conditions.Get(host, clusterv1.ReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1.NotReadyReason))
		Expect(ready.Message).To(ContainSubstring("server not found"))
	})

	It("RobotCredentialsAvailable=False takes priority over ProvisionSucceeded=False", func() {
		host := &infrav2.HetznerBareMetalHost{}

		conditions.Set(host, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostRobotCredentialsAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostRobotCredentialsInvalidReason,
			Message: "invalid credentials",
		})
		conditions.Set(host, metav1.Condition{
			Type:    infrav2.HetznerBareMetalHostProvisionSucceededCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav2.HetznerBareMetalHostServerNotFoundReason,
			Message: "server not found",
		})

		SetHetznerBareMetalHostReadySummary(host)

		ready := conditions.Get(host, clusterv1.ReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(clusterv1.NotReadyReason))
		// The summary lists all failing conditions in priority order. RobotCredentialsAvailable
		// (priority 1) must appear before ProvisionSucceeded (priority 5).
		Expect(ready.Message).To(ContainSubstring("invalid credentials"))
		Expect(ready.Message).To(ContainSubstring("server not found"))
		Expect(strings.Index(ready.Message, "invalid credentials")).
			To(BeNumerically("<", strings.Index(ready.Message, "server not found")))
	})
})
