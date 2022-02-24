package host

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
	sshmock "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/mocks/ssh"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

var _ = Describe("obtainHardwareDetailsNics", func() {
	DescribeTable("Complete successfully",
		func(stdout string, expectedOutput []infrav1.NIC) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHardwareDetailsNics").Return(sshclient.Output{StdOut: stdout})

			host := bareMetalHost()

			service := newTestService(host, nil, &sshMock, nil)

			Expect(service.obtainHardwareDetailsNics()).Should(Equal(expectedOutput))
		},
		Entry(
			"proper response",
			`name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ipv4="23.88.6.239/26" speedMbps="1000"
	name="eth0" model="Realtek Semiconductor Co." mac="a8:a1:59:94:19:42" ipv4="2a01:4f8:272:3e0f::2/64" speedMbps="1000"`,
			[]infrav1.NIC{
				{
					Name:      "eth0",
					Model:     "Realtek Semiconductor Co.",
					MAC:       "a8:a1:59:94:19:42",
					IP:        "23.88.6.239/26",
					SpeedMbps: 1000,
				}, {
					Name:      "eth0",
					Model:     "Realtek Semiconductor Co.",
					MAC:       "a8:a1:59:94:19:42",
					IP:        "2a01:4f8:272:3e0f::2/64",
					SpeedMbps: 1000,
				},
			}),
	)
})

var _ = Describe("obtainHardwareDetailsStorage", func() {
	DescribeTable("Complete successfully",
		func(stdout string, expectedOutput []infrav1.Storage) {
			sshMock := sshmock.Client{}
			sshMock.On("GetHardwareDetailsStorage").Return(sshclient.Output{StdOut: stdout})

			host := bareMetalHost()

			service := newTestService(host, nil, &sshMock, nil)

			Expect(service.obtainHardwareDetailsStorage()).Should(Equal(expectedOutput))
		},
		Entry(
			"proper response",
			`NAME="loop0" TYPE="loop" HCTL="" MODEL="" VENDOR="" SERIAL="" SIZE="3068773888" WWN="" ROTA="0"
NAME="nvme2n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVL22T0HBLB-00B00" VENDOR="" SERIAL="S677NF0R402742" SIZE="2048408248320" WWN="eui.002538b411b2cee8" ROTA="0"
NAME="nvme1n1" TYPE="disk" HCTL="" MODEL="SAMSUNG MZVLB512HAJQ-00000" VENDOR="" SERIAL="S3W8NX0N811178" SIZE="512110190592" WWN="eui.0025388801b4dff2" ROTA="0"`,
			[]infrav1.Storage{
				{
					Name:         "loop0",
					HCTL:         "",
					Model:        "",
					Vendor:       "",
					SerialNumber: "",
					SizeBytes:    3068773888,
					WWN:          "",
					Rota:         false,
				},
				{
					Name:         "nvme2n1",
					HCTL:         "",
					Model:        "SAMSUNG MZVL22T0HBLB-00B00",
					Vendor:       "",
					SerialNumber: "S677NF0R402742",
					SizeBytes:    2048408248320,
					WWN:          "eui.002538b411b2cee8",
					Rota:         false,
				},
				{
					Name:         "nvme1n1",
					HCTL:         "",
					Model:        "SAMSUNG MZVLB512HAJQ-00000",
					Vendor:       "",
					SerialNumber: "S3W8NX0N811178",
					SizeBytes:    512110190592,
					WWN:          "eui.0025388801b4dff2",
					Rota:         false,
				},
			}),
	)
})
