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

package hcloudutil

import (
	"errors"
	"testing"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

func TestHCloudUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HCloud utils tests")
}

var _ = Describe("Test ProviderIDFromServerID", func() {
	It("generates a correct providerID from a serverID", func() {
		Expect(ProviderIDFromServerID(42)).To(Equal("hcloud://42"))
	})
})

var _ = Describe("Test ServerIDFromProviderID", func() {
	It("gives the correct error when providerID is nil", func() {
		_, err := ServerIDFromProviderID(nil)
		Expect(err).ToNot(BeNil())
		Expect(err).To(MatchError(ErrNilProviderID))
	})

	type testCaseServerIDFromProviderID struct {
		providerID     string
		expectServerID int64
		expectError    error
	}

	DescribeTable("Test ServerIDFromProviderID",
		func(tc testCaseServerIDFromProviderID) {
			serverID, err := ServerIDFromProviderID(&tc.providerID)
			Expect(serverID).Should(Equal(tc.expectServerID))

			if tc.expectError != nil {
				Expect(err).ToNot(BeNil())
				Expect(err).Should(MatchError(tc.expectError))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("valid providerID", testCaseServerIDFromProviderID{
			providerID:     "hcloud://42",
			expectServerID: 42,
			expectError:    nil,
		}),
		Entry("invalid serverID", testCaseServerIDFromProviderID{
			providerID:     "hcloud://serverID",
			expectServerID: 0,
			expectError:    ErrInvalidProviderID,
		}),
		Entry("invalid providerID 1", testCaseServerIDFromProviderID{
			providerID:     "hcloud::serverID",
			expectServerID: 0,
			expectError:    ErrInvalidProviderID,
		}),
		Entry("invalid providerID 2", testCaseServerIDFromProviderID{
			providerID:     "hcloud://serverID://s",
			expectServerID: 0,
			expectError:    ErrInvalidProviderID,
		}),
	)
})

var _ = Describe("HandleRateLimitExceeded", func() {
	It("marks the object unreachable when rate limit is exceeded", func() {
		machine := &infrav1.HCloudMachine{}

		result := HandleRateLimitExceeded(machine, hcloud.Error{Code: hcloud.ErrorCodeRateLimitExceeded}, "ListServers")
		Expect(result).To(BeTrue())

		condition := conditions.Get(machine, infrav1.HetznerAPIReachableCondition)
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		Expect(condition.Reason).To(Equal(infrav1.RateLimitExceededReason))
		Expect(condition.Message).To(ContainSubstring("ListServers"))
	})

	It("returns false without touching conditions for other errors", func() {
		machine := &infrav1.HCloudMachine{}

		result := HandleRateLimitExceeded(machine, errors.New("other error"), "ListServers")
		Expect(result).To(BeFalse())
		Expect(conditions.Has(machine, infrav1.HetznerAPIReachableCondition)).To(BeFalse())
	})
})
