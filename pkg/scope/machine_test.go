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

package scope

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("Test ServerIDFromProviderID", func() {
	It("gives error on nil providerID", func() {
		hcloudMachine := infrav1.HCloudMachine{}
		machineScope := MachineScope{HCloudMachine: &hcloudMachine}

		serverID, err := machineScope.ServerIDFromProviderID()
		Expect(err).ToNot(BeNil())
		Expect(err).To(MatchError(ErrEmptyProviderID))
		Expect(serverID).To(Equal(int64(0)))
	})

	type testCaseServerIDFromProviderID struct {
		providerID     string
		expectServerID int64
		expectError    error
	}

	DescribeTable("Test ServerIDFromProviderID",
		func(tc testCaseServerIDFromProviderID) {
			hcloudMachine := infrav1.HCloudMachine{}
			hcloudMachine.Spec.ProviderID = &tc.providerID

			machineScope := MachineScope{HCloudMachine: &hcloudMachine}

			serverID, err := machineScope.ServerIDFromProviderID()

			if tc.expectError != nil {
				Expect(err).To(MatchError(tc.expectError))
			} else {
				Expect(err).To(BeNil())
			}
			Expect(serverID).Should(Equal(tc.expectServerID))
		},
		Entry("empty providerID", testCaseServerIDFromProviderID{
			providerID:     "",
			expectServerID: 0,
			expectError:    ErrEmptyProviderID,
		}),
		Entry("wrong prefix", testCaseServerIDFromProviderID{
			providerID:     "hclou://42",
			expectServerID: 0,
			expectError:    ErrInvalidProviderID,
		}),
		Entry("no prefix", testCaseServerIDFromProviderID{
			providerID:     "42",
			expectServerID: 0,
			expectError:    ErrInvalidProviderID,
		}),
		Entry("no serverID", testCaseServerIDFromProviderID{
			providerID:     "hcloud://",
			expectServerID: 0,
			expectError:    ErrInvalidServerID,
		}),
		Entry("invalid serverID - no int", testCaseServerIDFromProviderID{
			providerID:     "hcloud://serverID",
			expectServerID: 0,
			expectError:    ErrInvalidServerID,
		}),
		Entry("correct providerID", testCaseServerIDFromProviderID{
			providerID:     "hcloud://42",
			expectServerID: 42,
			expectError:    nil,
		}),
	)
})

var _ = Describe("SetHCloudMachineV1Beta2SummaryCondition", func() {
	It("lists all unhealthy conditions in priority order in the summary message", func() {
		hcloudMachine := &infrav1.HCloudMachine{
			Status: infrav1.HCloudMachineStatus{
				V1Beta2: &infrav1.HCloudMachineV1Beta2Status{},
			},
		}

		hcloudMachine.SetV1Beta2Conditions([]metav1.Condition{
			// ServerAvailable=False (lowest priority issue).
			{
				Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineServerNotFoundV1Beta2Reason,
				Message: "server is not available",
			},
			// Set HCloudTokenAvailable=False (highest priority issue).
			{
				Type:    infrav1.HCloudTokenAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudTokenInvalidV1Beta2Reason,
				Message: "token is invalid",
			},
		})

		Expect(SetHCloudMachineV1Beta2SummaryCondition(hcloudMachine)).To(Succeed())

		readyCond := hcloudMachine.GetV1Beta2Conditions()
		var summaryMsg string
		for _, c := range readyCond {
			if c.Type == infrav1.HCloudMachineReadyV1Beta2Condition {
				summaryMsg = c.Message
				Expect(c.Status).To(Equal(metav1.ConditionFalse))
				break
			}
		}
		Expect(summaryMsg).ToNot(BeEmpty(), "Ready summary condition should have a message")

		// The summary message lists all unhealthy conditions in ForConditionTypes order.
		// HCloudTokenAvailable (priority 1) before ServerAvailable (priority 5).
		Expect(summaryMsg).To(MatchRegexp(`(?s)token is invalid.*server is not available`))
	})

	It("surfaces RateLimitExceeded before ServerAvailable when both are unhealthy", func() {
		hcloudMachine := &infrav1.HCloudMachine{
			Status: infrav1.HCloudMachineStatus{
				V1Beta2: &infrav1.HCloudMachineV1Beta2Status{},
			},
		}

		hcloudMachine.SetV1Beta2Conditions([]metav1.Condition{
			// HCloudRateLimitExceeded=True (negative polarity, priority 2).
			{
				Type:    infrav1.HCloudRateLimitExceededV1Beta2Condition,
				Status:  metav1.ConditionTrue,
				Reason:  infrav1.HCloudRateLimitExceededV1Beta2Reason,
				Message: "rate limit exceeded",
			},
			// ServerAvailable=False with Deleting reason (priority 5).
			{
				Type:    infrav1.HCloudMachineServerAvailableV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.HCloudMachineDeletingV1Beta2Reason,
				Message: "machine is deleting",
			},
		})

		Expect(SetHCloudMachineV1Beta2SummaryCondition(hcloudMachine)).To(Succeed())

		readyCond := hcloudMachine.GetV1Beta2Conditions()
		var summaryMsg string
		for _, c := range readyCond {
			if c.Type == infrav1.HCloudMachineReadyV1Beta2Condition {
				summaryMsg = c.Message
				break
			}
		}
		Expect(summaryMsg).ToNot(BeEmpty())

		// HCloudRateLimitExceeded (priority 2) before ServerAvailable (priority 5).
		Expect(summaryMsg).To(MatchRegexp(`(?s)rate limit exceeded.*machine is deleting`))
	})
})
