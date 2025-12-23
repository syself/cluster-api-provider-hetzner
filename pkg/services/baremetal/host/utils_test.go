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

package host

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
)

var _ = Describe("buildAutoSetup", func() {
	type testCaseBuildAutoSetup struct {
		installImageSpec *infrav1.InstallImage
		asi              autoSetupInput
		expectedOutput   string
	}
	DescribeTable("buildAutoSetup",
		func(tc testCaseBuildAutoSetup) {
			Expect(buildAutoSetup(tc.installImageSpec, tc.asi)).Should(Equal(tc.expectedOutput))
		},
		Entry("multiple entries", testCaseBuildAutoSetup{
			installImageSpec: &infrav1.InstallImage{
				Partitions: []infrav1.Partition{
					{
						Mount:      "/boot",
						FileSystem: "ext2",
						Size:       "512M",
					},
					{
						Mount:      "swap",
						FileSystem: "swap",
						Size:       "4G",
					},
				},
				LVMDefinitions: []infrav1.LVMDefinition{
					{
						VG:         "vg0",
						Name:       "root",
						Mount:      "/",
						FileSystem: "ext4",
						Size:       "10G",
					},
					{
						VG:         "vg0",
						Name:       "swap",
						Mount:      "swap",
						FileSystem: "swap",
						Size:       "5G",
					},
				},
				BTRFSDefinitions: []infrav1.BTRFSDefinition{
					{
						Volume:    "btrfs.1",
						SubVolume: "@",
						Mount:     "/",
					},
					{
						Volume:    "btrfs.1",
						SubVolume: "@/usr",
						Mount:     "/usr",
					},
				},
				Swraid:      0,
				SwraidLevel: 1,
			},
			asi: autoSetupInput{
				image:     "my-image",
				osDevices: []string{"device"},
				hostName:  "my-host",
			},
			expectedOutput: `DRIVE1 /dev/device

HOSTNAME my-host
SWRAID 0

PART /boot ext2 512M
PART swap swap 4G

LV vg0 root / ext4 10G
LV vg0 swap swap swap 5G

SUBVOL btrfs.1 @ /
SUBVOL btrfs.1 @/usr /usr

IMAGE my-image`,
		}),
		Entry("single entries", testCaseBuildAutoSetup{
			installImageSpec: &infrav1.InstallImage{
				Partitions: []infrav1.Partition{
					{
						Mount:      "/boot",
						FileSystem: "ext2",
						Size:       "512M",
					},
				},
				LVMDefinitions: []infrav1.LVMDefinition{
					{
						VG:         "vg0",
						Name:       "root",
						Mount:      "/",
						FileSystem: "ext4",
						Size:       "10G",
					},
				},
				BTRFSDefinitions: []infrav1.BTRFSDefinition{
					{
						Volume:    "btrfs.1",
						SubVolume: "@",
						Mount:     "/",
					},
				},
				Swraid:      1,
				SwraidLevel: 1,
			},
			asi: autoSetupInput{
				image:     "my-image",
				osDevices: []string{"device"},
				hostName:  "my-host",
			},
			expectedOutput: `DRIVE1 /dev/device

HOSTNAME my-host
SWRAID 1
SWRAIDLEVEL 1

PART /boot ext2 512M

LV vg0 root / ext4 10G

SUBVOL btrfs.1 @ /

IMAGE my-image`,
		}),
		Entry("multiple drives", testCaseBuildAutoSetup{
			installImageSpec: &infrav1.InstallImage{
				Partitions: []infrav1.Partition{
					{
						Mount:      "/boot",
						FileSystem: "ext2",
						Size:       "512M",
					},
				},
				LVMDefinitions: []infrav1.LVMDefinition{
					{
						VG:         "vg0",
						Name:       "root",
						Mount:      "/",
						FileSystem: "ext4",
						Size:       "10G",
					},
				},
				BTRFSDefinitions: []infrav1.BTRFSDefinition{
					{
						Volume:    "btrfs.1",
						SubVolume: "@",
						Mount:     "/",
					},
				},
				Swraid:      0,
				SwraidLevel: 1,
			},
			asi: autoSetupInput{
				image:     "my-image",
				osDevices: []string{"device1", "device2"},
				hostName:  "my-host",
			},
			expectedOutput: `DRIVE1 /dev/device1
DRIVE2 /dev/device2

HOSTNAME my-host
SWRAID 0

PART /boot ext2 512M

LV vg0 root / ext4 10G

SUBVOL btrfs.1 @ /

IMAGE my-image`,
		}),
		Entry("proper response", testCaseBuildAutoSetup{
			installImageSpec: &infrav1.InstallImage{
				Partitions: []infrav1.Partition{
					{
						Mount:      "/boot",
						FileSystem: "ext2",
						Size:       "512M",
					},
				},
				LVMDefinitions:   []infrav1.LVMDefinition{},
				BTRFSDefinitions: []infrav1.BTRFSDefinition{},
				Swraid:           0,
				SwraidLevel:      1,
			},
			asi: autoSetupInput{
				image:     "my-image",
				osDevices: []string{"device"},
				hostName:  "my-host",
			},
			expectedOutput: `DRIVE1 /dev/device

HOSTNAME my-host
SWRAID 0

PART /boot ext2 512M



IMAGE my-image`,
		}),
	)
})

var _ = Describe("validJSONFromSSHOutput", func() {
	type testCaseValidJSONFromSSHOutput struct {
		input          string
		expectedOutput string
	}
	DescribeTable("validJSONFromSSHOutput", func(tc testCaseValidJSONFromSSHOutput) {
		Expect(validJSONFromSSHOutput(tc.input)).Should(Equal(tc.expectedOutput))
	},
		Entry("working example", testCaseValidJSONFromSSHOutput{
			input:          `key1="string1" key2="string2" key3="string3"`,
			expectedOutput: `{"key1":"string1","key2":"string2","key3":"string3"}`,
		}),
		Entry("working example2", testCaseValidJSONFromSSHOutput{
			input:          `key1="string1"`,
			expectedOutput: `{"key1":"string1"}`,
		}),
		Entry("empty string", testCaseValidJSONFromSSHOutput{
			input:          ``,
			expectedOutput: `{}`,
		}),
	)
})

var _ = Describe("Test splitHostKey", func() {
	It("Should split the host key", func() {
		namespace, name := splitHostKey("namespace/name")
		Expect(namespace).To(Equal("namespace"))
		Expect(name).To(Equal("name"))
	})
})
