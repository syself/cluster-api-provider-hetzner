/*
Copyright 2022 The Kubernetes Authors.

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

package server

import (
	"github.com/hetznercloud/hcloud-go/hcloud"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

var _ = Describe("setStatusFromAPI", func() {
	var sts infrav1.HCloudMachineStatus
	BeforeEach(func() {
		sts = setStatusFromAPI(server)
	})
	It("should have the right instance state", func() {
		Expect(*sts.InstanceState).To(Equal(instanceState))

	})
	It("should have three addresses", func() {
		Expect(len(sts.Addresses)).To(Equal(3))
	})
	It("should have the right address IPs", func() {
		for i, addr := range sts.Addresses {
			Expect(addr.Address).To(Equal(ips[i]))
		}
	})
	It("should have the right address types", func() {
		for i, addr := range sts.Addresses {
			Expect(addr.Type).To(Equal(addressTypes[i]))
		}
	})
})

var _ = DescribeTable("createLabels",
	func(hcloudClusterName, hcloudMachineName string, isControlPlane bool, expectedOutput map[string]string) {
		Expect(createLabels(hcloudClusterName, hcloudMachineName, isControlPlane)).To(Equal(expectedOutput))
	},
	Entry("is_controlplane", "hcloudCluster", "hcloudMachine", true, map[string]string{
		infrav1.ClusterTagKey("hcloudCluster"): string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:              "hcloudMachine",
		"machine_type":                         "control_plane",
	}),
	Entry("is_worker", "hcloudCluster", "hcloudMachine", false, map[string]string{
		infrav1.ClusterTagKey("hcloudCluster"): string(infrav1.ResourceLifecycleOwned),
		infrav1.MachineNameTagKey:              "hcloudMachine",
		"machine_type":                         "worker",
	}),
)

var _ = Describe("getSSHKeys", func() {
	var sshKeysAPI []*hcloud.SSHKey
	BeforeEach(func() {
		sshKeysAPI = []*hcloud.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
				ID:          42,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:3g",
				Name:        "sshkey2",
				ID:          43,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4h",
				Name:        "sshkey3",
				ID:          44,
			},
		}
	})
	var _ = DescribeTable("no_error",
		func(sshKeysSpec []infrav1.SSHKey, expectedOutput []*hcloud.SSHKey) {
			Expect(getSSHKeys(sshKeysAPI, sshKeysSpec)).Should(Equal(expectedOutput))
		},
		Entry("no_error_same_length", []infrav1.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:3g",
				Name:        "sshkey2",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
				Name:        "sshkey3",
			},
		}, []*hcloud.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
				ID:          42,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:3g",
				Name:        "sshkey2",
				ID:          43,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4h",
				Name:        "sshkey3",
				ID:          44,
			},
		}),
		Entry("no_error_different_length", []infrav1.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4f",
				Name:        "sshkey3",
			},
		}, []*hcloud.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
				ID:          42,
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:4h",
				Name:        "sshkey3",
				ID:          44,
			},
		}),
	)

	It("should error", func() {
		_, err := getSSHKeys(sshKeysAPI, []infrav1.SSHKey{
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f",
				Name:        "sshkey1",
			},
			{
				Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:5i",
				Name:        "sshkey4",
			},
		})
		Expect(err).To(HaveOccurred())
	})
})
