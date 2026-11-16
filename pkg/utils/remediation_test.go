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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"

	"github.com/syself/cluster-api-provider-hetzner/pkg/utils"
)

var _ = DescribeTable("MachineRecoveredAfterReboot",
	func(nodeHealthy, healthCheck metav1.ConditionStatus, want bool) {
		machine := &clusterv1.Machine{}
		if nodeHealthy != "" {
			conditions.Set(machine, metav1.Condition{Type: clusterv1.MachineNodeHealthyCondition, Status: nodeHealthy, Reason: "Test"})
		}
		if healthCheck != "" {
			conditions.Set(machine, metav1.Condition{Type: clusterv1.MachineHealthCheckSucceededCondition, Status: healthCheck, Reason: "Test"})
		}
		Expect(utils.MachineRecoveredAfterReboot(machine)).To(Equal(want))
	},
	Entry("Node healthy and health check passes", metav1.ConditionTrue, metav1.ConditionTrue, true),
	Entry("Node healthy and no health check condition", metav1.ConditionTrue, metav1.ConditionStatus(""), true),
	Entry("Node healthy but a custom Node condition still fails the health check", metav1.ConditionTrue, metav1.ConditionFalse, false),
	Entry("Node not healthy", metav1.ConditionFalse, metav1.ConditionTrue, false),
	Entry("no conditions", metav1.ConditionStatus(""), metav1.ConditionStatus(""), false),
)
