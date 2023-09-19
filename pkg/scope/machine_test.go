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
