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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav2 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	fakehcloudclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/hcloud/client/fake"
)

var _ = Describe("Test ServerIDFromProviderID", func() {
	It("gives error on nil providerID", func() {
		hcloudMachine := infrav2.HCloudMachine{}
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
			hcloudMachine := infrav2.HCloudMachine{}
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

var _ = Describe("HCloudMachineSummaryOpts", func() {
	It("lists all unhealthy conditions in priority order in the summary message", func() {
		hcloudMachine := &infrav2.HCloudMachine{}

		hcloudMachine.SetConditions([]metav1.Condition{
			// ServerAvailable=False (lowest priority issue).
			{
				Type:    infrav2.HCloudMachineServerAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudMachineServerNotFoundReason,
				Message: "server is not available",
			},
			// HCloudTokenAvailable=False (highest priority issue).
			{
				Type:    infrav2.HCloudTokenAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudTokenInvalidReason,
				Message: "token is invalid",
			},
		})

		readyCondition, err := conditions.NewSummaryCondition(hcloudMachine, clusterv1.ReadyCondition, infrav2.HCloudMachineSummaryOpts()...)
		Expect(err).To(BeNil())
		Expect(readyCondition).ToNot(BeNil())
		Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))

		// The summary message lists all unhealthy conditions in ForConditionTypes order.
		// HCloudTokenAvailable (priority 1) before ServerAvailable (priority 5).
		Expect(readyCondition.Message).To(MatchRegexp(`(?s)token is invalid.*server is not available`))
	})

	It("surfaces RateLimitExceeded before ServerAvailable when both are unhealthy", func() {
		hcloudMachine := &infrav2.HCloudMachine{}

		hcloudMachine.SetConditions([]metav1.Condition{
			// HCloudRateLimitExceeded=True (negative polarity, priority 2).
			{
				Type:    infrav2.HCloudRateLimitExceededCondition,
				Status:  metav1.ConditionTrue,
				Reason:  infrav2.HCloudRateLimitExceededReason,
				Message: "rate limit exceeded",
			},
			// ServerAvailable=False with Deleting reason (priority 5).
			{
				Type:    infrav2.HCloudMachineServerAvailableCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav2.HCloudMachineDeletingReason,
				Message: "machine is deleting",
			},
		})

		readyCondition, err := conditions.NewSummaryCondition(hcloudMachine, clusterv1.ReadyCondition, infrav2.HCloudMachineSummaryOpts()...)
		Expect(err).To(BeNil())
		Expect(readyCondition).ToNot(BeNil())

		// HCloudRateLimitExceeded (priority 2) before ServerAvailable (priority 5).
		Expect(readyCondition.Message).To(MatchRegexp(`(?s)rate limit exceeded.*machine is deleting`))
	})
})

var _ = Describe("NewMachineScope with a missing owner Machine", func() {
	newParams := func(hcloudMachine *infrav2.HCloudMachine) MachineScopeParams {
		scheme := runtime.NewScheme()
		utilruntime.Must(clusterv1.AddToScheme(scheme))
		utilruntime.Must(infrav2.AddToScheme(scheme))
		crClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

		return MachineScopeParams{
			Client:         crClient,
			APIReader:      crClient,
			Logger:         klog.Background(),
			HCloudClient:   fakehcloudclient.NewHCloudClientFactory().NewClient(""),
			Cluster:        &clusterv1.Cluster{},
			HetznerCluster: &infrav2.HetznerCluster{},
			HCloudMachine:  hcloudMachine,
		}
	}

	It("fails when the HCloudMachine is not being deleted", func() {
		hcloudMachine := &infrav2.HCloudMachine{}

		_, err := NewMachineScope(newParams(hcloudMachine))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("nil Machine"))
	})

	It("succeeds when the HCloudMachine is being deleted", func() {
		now := metav1.Now()
		hcloudMachine := &infrav2.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "hcloud-machine",
				Namespace:         "default",
				DeletionTimestamp: &now,
				Finalizers:        []string{infrav2.HCloudMachineFinalizer},
			},
		}

		machineScope, err := NewMachineScope(newParams(hcloudMachine))
		Expect(err).ToNot(HaveOccurred())
		Expect(machineScope.Machine).To(BeNil())
	})
})

var _ = Describe("IsControlPlane without an owner Machine", func() {
	It("reads the label from the HCloudMachine", func() {
		hcloudMachine := &infrav2.HCloudMachine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{clusterv1.MachineControlPlaneLabel: ""},
			},
		}
		machineScope := MachineScope{HCloudMachine: hcloudMachine}

		Expect(machineScope.IsControlPlane()).To(BeTrue())
	})

	It("is false for a worker", func() {
		machineScope := MachineScope{HCloudMachine: &infrav2.HCloudMachine{}}

		Expect(machineScope.IsControlPlane()).To(BeFalse())
	})

	It("still uses the Machine when it is set", func() {
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{clusterv1.MachineControlPlaneLabel: ""},
			},
		}
		machineScope := MachineScope{Machine: machine, HCloudMachine: &infrav2.HCloudMachine{}}

		Expect(machineScope.IsControlPlane()).To(BeTrue())
	})
})
