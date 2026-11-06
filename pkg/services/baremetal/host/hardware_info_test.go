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

package host

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	bmmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
	"github.com/syself/cluster-api-provider-hetzner/test/helpers"
)

// ex44HardwareInfo is the output of hardware-info.sh in the rescue system of an EX44.
// The flags line is shortened to the flags that x86Level and amx read.
const ex44HardwareInfo = `board_vendor=ASUSTeK COMPUTER INC.
board_name=PRIME B760M-A D4
board_version=Rev 1.xx
firmware_vendor=American Megatrends Inc.
firmware_version=9017
boot_mode=UEFI
cpu_family=6
cpu_model_id=191
cpu_stepping=2
cpu_microcode=0x3d
cpu_flags=fpu lm cx16 lahf_lm popcnt sse4_1 sse4_2 ssse3 avx avx2 bmi1 bmi2 f16c fma abm movbe xsave
memory_type=DDR4
memory_ecc=None
watchdog_driver=iTCO_wdt
pci_device=0000:00:02.0 0x8086 0x4680 0x030000 i915
pci_device=0000:00:17.0 0x8086 0x7a62 0x010601 ahci
pci_device=0000:01:00.0 0x144d 0xa80a 0x010802 nvme
pci_device=0000:05:00.0 0x10ec 0x8125 0x020000 r8169
`

// ax41HardwareInfo is the output of hardware-info.sh in the rescue system of an AX41-NVMe. The
// firmware does not report a board version, and one SATA controller has no driver.
const ax41HardwareInfo = `board_vendor=ASRock
board_name=B450 Pro4 R2.0
board_version=
firmware_vendor=American Megatrends Inc.
firmware_version=H8.02E
boot_mode=Legacy
cpu_family=23
cpu_model_id=113
cpu_stepping=0
cpu_microcode=0x8701034
cpu_flags=fpu lm cx16 lahf_lm popcnt sse4_1 sse4_2 ssse3 avx avx2 bmi1 bmi2 f16c fma abm movbe xsave
memory_type=DDR4
memory_ecc=None
watchdog_driver=sp5100-tco
pci_device=0000:08:00.0 0x1b21 0x0612 0x010601
pci_device=0000:09:00.0 0x10ec 0x8168 0x020000 r8169
`

// registeringStorageStdOut has one disk with the WWN of helpers.WithRootDeviceHintWWN.
const registeringStorageStdOut = `NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"`

var _ = Describe("parseHardwareInfo", func() {
	It("sets every value of an EX44", func() {
		var hardwareDetails infrav1.HardwareDetails

		Expect(parseHardwareInfo(ex44HardwareInfo, &hardwareDetails)).To(Succeed())

		Expect(hardwareDetails.Board).To(Equal(infrav1.Board{
			Vendor:  "ASUSTeK COMPUTER INC.",
			Name:    "PRIME B760M-A D4",
			Version: "Rev 1.xx",
		}))
		Expect(hardwareDetails.Firmware).To(Equal(infrav1.Firmware{
			Vendor:   "American Megatrends Inc.",
			Version:  "9017",
			BootMode: infrav1.BootModeUEFI,
		}))
		Expect(hardwareDetails.CPU).To(Equal(infrav1.CPU{
			Family:    6,
			ModelID:   ptr.To[int32](191),
			Stepping:  ptr.To[int32](2),
			Microcode: "0x3d",
			X86Level:  3,
			AMX:       infrav1.CPUFeatureUnsupported,
		}))
		Expect(hardwareDetails.Memory).To(Equal(infrav1.Memory{
			Type: "DDR4",
			ECC:  infrav1.ECCDisabled,
		}))
		Expect(hardwareDetails.WatchdogDriver).To(Equal("iTCO_wdt"))
		Expect(hardwareDetails.PCIDevices).To(Equal([]infrav1.PCIDevice{
			{Address: "0000:00:02.0", VendorID: "0x8086", DeviceID: "0x4680", Class: "0x030000", Driver: "i915"},
			{Address: "0000:00:17.0", VendorID: "0x8086", DeviceID: "0x7a62", Class: "0x010601", Driver: "ahci"},
			{Address: "0000:01:00.0", VendorID: "0x144d", DeviceID: "0xa80a", Class: "0x010802", Driver: "nvme"},
			{Address: "0000:05:00.0", VendorID: "0x10ec", DeviceID: "0x8125", Class: "0x020000", Driver: "r8169"},
		}))
	})

	It("leaves empty values empty and keeps stepping 0 of an AX41-NVMe", func() {
		var hardwareDetails infrav1.HardwareDetails

		Expect(parseHardwareInfo(ax41HardwareInfo, &hardwareDetails)).To(Succeed())

		Expect(hardwareDetails.Board.Version).To(BeEmpty())
		Expect(hardwareDetails.Firmware.BootMode).To(Equal(infrav1.BootModeLegacy))
		Expect(hardwareDetails.CPU.Stepping).To(Equal(ptr.To[int32](0)))
		Expect(hardwareDetails.PCIDevices).To(Equal([]infrav1.PCIDevice{
			{Address: "0000:08:00.0", VendorID: "0x1b21", DeviceID: "0x0612", Class: "0x010601"},
			{Address: "0000:09:00.0", VendorID: "0x10ec", DeviceID: "0x8168", Class: "0x020000", Driver: "r8169"},
		}))
	})

	It("returns an error for a value it cannot parse and still sets the other values", func() {
		var hardwareDetails infrav1.HardwareDetails

		err := parseHardwareInfo("board_name=PRIME B760M-A D4\ncpu_family=six\npci_device=0000:05:00.0\n", &hardwareDetails)

		Expect(err).To(MatchError(ContainSubstring(`failed to parse cpu_family "six"`)))
		Expect(err).To(MatchError(ContainSubstring(`failed to parse pci_device "0000:05:00.0"`)))
		Expect(hardwareDetails.Board.Name).To(Equal("PRIME B760M-A D4"))
		Expect(hardwareDetails.CPU.Family).To(BeZero())
		Expect(hardwareDetails.PCIDevices).To(BeEmpty())
	})

	DescribeTable("leaves a field empty when its value would break the API limits",
		func(stdOut, wantErr string) {
			var hardwareDetails infrav1.HardwareDetails
			Expect(parseHardwareInfo(stdOut, &hardwareDetails)).To(MatchError(ContainSubstring(wantErr)))
			Expect(hardwareDetails).To(BeZero())
		},
		Entry("board name over 256 characters", "board_name="+strings.Repeat("a", 257), "board_name is longer than 256 characters"),
		Entry("negative stepping", "cpu_stepping=-1", "cpu_stepping -1 is below the minimum 0"),
		Entry("family 0", "cpu_family=0", "cpu_family 0 is below the minimum 1"),
		Entry("unknown boot mode", "boot_mode=EFI", `unknown boot_mode "EFI"`),
		Entry("PCI driver over 64 characters", "pci_device=0000:05:00.0 0x10ec 0x8125 0x020000 "+strings.Repeat("d", 65), "longer than the API allows"),
	)

	DescribeTable("memory_ecc",
		func(value string, want infrav1.ECCState) {
			var hardwareDetails infrav1.HardwareDetails
			Expect(parseHardwareInfo("memory_ecc="+value, &hardwareDetails)).To(Succeed())
			Expect(hardwareDetails.Memory.ECC).To(Equal(want))
		},
		Entry("None means disabled", "None", infrav1.ECCDisabled),
		Entry("Multi-bit ECC means enabled", "Multi-bit ECC", infrav1.ECCEnabled),
		Entry("Single-bit ECC means enabled", "Single-bit ECC", infrav1.ECCEnabled),
		Entry("Unknown stays empty", "Unknown", infrav1.ECCState("")),
	)

	It("sets amx when the CPU has the flag amx_tile", func() {
		var hardwareDetails infrav1.HardwareDetails
		Expect(parseHardwareInfo("cpu_flags=lm amx_tile", &hardwareDetails)).To(Succeed())
		Expect(hardwareDetails.CPU.AMX).To(Equal(infrav1.CPUFeatureSupported))
	})

	It("skips the memory type Unknown of an empty slot", func() {
		var hardwareDetails infrav1.HardwareDetails
		Expect(parseHardwareInfo("memory_type=Unknown", &hardwareDetails)).To(Succeed())
		Expect(hardwareDetails.Memory.Type).To(BeEmpty())
	})
})

var _ = DescribeTable("x86Level",
	func(flags []string, want int32) {
		Expect(x86Level(flags)).To(Equal(want))
	},
	Entry("no long mode means no x86-64 level", []string{"fpu", "sse2"}, int32(0)),
	Entry("long mode only is level 1", []string{"lm"}, int32(1)),
	Entry("level 2 flags", append([]string{"lm"}, x86LevelFlags[0]...), int32(2)),
	Entry("level 3 flags", append(append([]string{"lm"}, x86LevelFlags[0]...), x86LevelFlags[1]...), int32(3)),
	Entry("level 4 flags", append(append(append([]string{"lm"}, x86LevelFlags[0]...), x86LevelFlags[1]...), x86LevelFlags[2]...), int32(4)),
	Entry("AVX-512 without the level 3 flags is level 2", append(append([]string{"lm"}, x86LevelFlags[0]...), x86LevelFlags[2]...), int32(2)),
)

var _ = Describe("actionRegistering with hardware-info.sh", func() {
	ctx := context.Background()

	// registeringHost returns a host and a service whose rescue system returns hardwareInfo for hardware-info.sh.
	registeringHost := func(hardwareInfo sshclient.Output) (*infrav1.HetznerBareMetalHost, *Service) {
		host := helpers.BareMetalHost(
			"test-host",
			"default",
			helpers.WithRootDeviceHintWWN(),
			helpers.WithIPv4(),
			helpers.WithConsumerRef(),
		)
		sshMock := registeringSSHMockWithHardwareInfo(registeringStorageStdOut, hardwareInfo)
		service := newTestService(host, nil, bmmock.NewSSHFactory(sshMock, sshMock, sshMock), nil, helpers.GetDefaultSSHSecret(rescueSSHKeyName, "default"))
		return host, service
	}

	It("stores the values of hardware-info.sh in hardwareDetails", func() {
		host, service := registeringHost(sshclient.Output{StdOut: ex44HardwareInfo})

		Expect(service.actionRegistering(ctx)).To(BeAssignableToTypeOf(actionComplete{}))

		Expect(host.Status.HardwareDetails.Board.Name).To(Equal("PRIME B760M-A D4"))
		Expect(host.Status.HardwareDetails.PCIDevices).To(HaveLen(4))
		// the values that getHardwareDetails reads are still set.
		Expect(host.Status.HardwareDetails.CPU.Model).To(Equal("mymodel"))
	})

	It("continues registering when hardware-info.sh fails", func() {
		host, service := registeringHost(sshclient.Output{
			StdOut: "board_name=PRIME B760M-A D4\nmemory_type=\n",
			StdErr: "dmidecode: command not found",
		})

		Expect(service.actionRegistering(ctx)).To(BeAssignableToTypeOf(actionComplete{}))

		Expect(host.Status.HardwareDetails.Board.Name).To(Equal("PRIME B760M-A D4"))
		Expect(host.Status.HardwareDetails.Memory).To(BeZero())
		Expect(host.Status.HardwareDetails.CPU.Model).To(Equal("mymodel"))
	})
})
