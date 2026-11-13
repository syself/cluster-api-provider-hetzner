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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"k8s.io/utils/ptr"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta2"
	sshclient "github.com/syself/cluster-api-provider-hetzner/pkg/services/baremetal/client/ssh"
)

// The MaxItems and MaxLength limits of hardwareDetails.pciDevices. A longer list or value would
// make the API server reject the whole status update.
const (
	maxPCIDevices       = 128
	maxPCIAddressLength = 32
	maxPCIIDLength      = 16
	maxPCIDriverLength  = 64
)

// maxValueLength has the MaxLength limits of the API fields that get a value as it is. A longer
// value would make the API server reject the whole status update.
var maxValueLength = map[string]int{
	"board_vendor":     256,
	"board_name":       256,
	"board_version":    256,
	"firmware_vendor":  256,
	"firmware_version": 256,
	"cpu_microcode":    32,
	"memory_type":      64,
	"watchdog_driver":  64,
}

// x86LevelFlags are the CPU flags that the x86-64 levels 2, 3 and 4 add, as /proc/cpuinfo names
// them. A CPU has a level when it has the flags of that level and of all levels below it.
// The kernel calls SSE3 "pni" and LZCNT "abm".
var x86LevelFlags = [][]string{
	{"cx16", "lahf_lm", "popcnt", "pni", "sse4_1", "sse4_2", "ssse3"},
	{"avx", "avx2", "bmi1", "bmi2", "f16c", "fma", "abm", "movbe", "xsave"},
	{"avx512f", "avx512bw", "avx512cd", "avx512dq", "avx512vl"},
}

// addHardwareInfo adds the board, firmware, CPU, memory, watchdog and PCI device details that
// hardware-info.sh prints to hardwareDetails. Nothing in the controller reads these values, so
// registering continues when they cannot be read, and the controller logs an error.
// When the ssh command fails, nothing was read, and the values from the last registering in
// previous are kept. When a single value cannot be read or parsed, its field stays empty.
func (s *Service) addHardwareInfo(ctx context.Context, sshClient sshclient.Client, previous, hardwareDetails *infrav1.HardwareDetails) {
	out := sshClient.GetHardwareInfo(ctx)

	if out.Err != nil {
		copyHardwareInfo(previous, hardwareDetails)
		s.scope.Error(out.Err, "failed to run hardware-info.sh, keeping the hardware details from the last registering")
		return
	}

	if err := errors.Join(handleSSHError(out), parseHardwareInfo(out.StdOut, hardwareDetails)); err != nil {
		s.scope.Error(err, "failed to read some hardware details, their fields stay empty", "stdout", out.StdOut)
	}
}

// copyHardwareInfo copies the values that addHardwareInfo sets from one hardwareDetails to another.
// It does nothing when from is nil.
func copyHardwareInfo(from, to *infrav1.HardwareDetails) {
	if from == nil {
		return
	}

	to.Board = from.Board
	to.Firmware = from.Firmware
	to.Memory = from.Memory
	to.WatchdogDriver = from.WatchdogDriver
	to.PCIDevices = slices.Clone(from.PCIDevices)

	to.CPU.Family = from.CPU.Family
	to.CPU.ModelID = from.CPU.ModelID
	to.CPU.Stepping = from.CPU.Stepping
	to.CPU.Microcode = from.CPU.Microcode
	to.CPU.X86Level = from.CPU.X86Level
	to.CPU.AMX = from.CPU.AMX
}

// parseHardwareInfo sets the fields of hardwareDetails from the key=value lines of hardware-info.sh.
// A key with an empty value leaves its field empty. It returns an error for each value it cannot parse.
func parseHardwareInfo(stdOut string, hardwareDetails *infrav1.HardwareDetails) error {
	var errs []error
	skippedPCIDevices := 0
	var memoryTotalWidth, memoryDataWidth, memoryErrorCorrection string

	for line := range strings.Lines(stdOut) {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if limit, ok := maxValueLength[key]; ok && len(value) > limit {
			errs = append(errs, fmt.Errorf("%s is longer than %d characters: %q", key, limit, value))
			continue
		}

		switch key {
		case "board_vendor":
			hardwareDetails.Board.Vendor = value
		case "board_name":
			hardwareDetails.Board.Name = value
		case "board_version":
			hardwareDetails.Board.Version = value
		case "firmware_vendor":
			hardwareDetails.Firmware.Vendor = value
		case "firmware_version":
			hardwareDetails.Firmware.Version = value
		case "boot_mode":
			bootMode := infrav1.BootMode(value)
			if bootMode != infrav1.BootModeUEFI && bootMode != infrav1.BootModeLegacy {
				errs = append(errs, fmt.Errorf("unknown boot_mode %q", value))
				continue
			}
			hardwareDetails.Firmware.BootMode = bootMode

		case "cpu_family":
			family, err := parseInt32(key, value, 1)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			hardwareDetails.CPU.Family = family
		case "cpu_model_id":
			modelID, err := parseInt32(key, value, 0)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			hardwareDetails.CPU.ModelID = ptr.To(modelID)
		case "cpu_stepping":
			stepping, err := parseInt32(key, value, 0)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			hardwareDetails.CPU.Stepping = ptr.To(stepping)
		case "cpu_microcode":
			hardwareDetails.CPU.Microcode = value
		case "cpu_flags":
			flags := strings.Fields(value)
			hardwareDetails.CPU.X86Level = x86Level(flags)
			hardwareDetails.CPU.AMX = infrav1.CPUFeatureUnsupported
			if slices.Contains(flags, "amx_tile") {
				hardwareDetails.CPU.AMX = infrav1.CPUFeatureSupported
			}

		case "memory_type":
			// dmidecode reports "Unknown" or "Other" when the firmware does not know the type.
			if value != "Unknown" && value != "Other" {
				hardwareDetails.Memory.Type = value
			}
		case "memory_total_width":
			memoryTotalWidth = value
		case "memory_data_width":
			memoryDataWidth = value
		case "memory_error_correction":
			memoryErrorCorrection = value

		case "watchdog_driver":
			hardwareDetails.WatchdogDriver = value

		case "pci_device":
			// the line has the address, vendor ID, device ID, class and driver. A device
			// without a driver has only the first four.
			fields := strings.Fields(value)
			if len(fields) < 4 {
				errs = append(errs, fmt.Errorf("failed to parse pci_device %q: want at least 4 fields", value))
				continue
			}
			if len(hardwareDetails.PCIDevices) == maxPCIDevices {
				skippedPCIDevices++
				continue
			}
			if len(fields[0]) > maxPCIAddressLength || len(fields[1]) > maxPCIIDLength || len(fields[2]) > maxPCIIDLength ||
				len(fields[3]) > maxPCIIDLength || (len(fields) > 4 && len(fields[4]) > maxPCIDriverLength) {
				errs = append(errs, fmt.Errorf("pci_device %q has a value that is longer than the API allows", value))
				continue
			}
			device := infrav1.PCIDevice{
				Address:  fields[0],
				VendorID: fields[1],
				DeviceID: fields[2],
				Class:    fields[3],
			}
			if len(fields) > 4 {
				device.Driver = fields[4]
			}
			hardwareDetails.PCIDevices = append(hardwareDetails.PCIDevices, device)
		}
	}

	hardwareDetails.Memory.ECC = eccState(memoryTotalWidth, memoryDataWidth, memoryErrorCorrection)

	if skippedPCIDevices > 0 {
		errs = append(errs, fmt.Errorf("found %d PCI devices, the last %d are left out", maxPCIDevices+skippedPCIDevices, skippedPCIDevices))
	}

	return errors.Join(errs...)
}

// eccState tells whether ECC memory is installed and whether error correction is on.
// totalWidth and dataWidth come from the first installed memory module, for example "72 bits" and
// "64 bits". errorCorrection is what the firmware reports for the memory array, for example
// "Multi-bit ECC" or "None". It returns an empty state when dmidecode does not know enough.
func eccState(totalWidth, dataWidth, errorCorrection string) infrav1.ECCState {
	// dmidecode prints one of these values for the memory array (SMBIOS type 16): "Other", "Unknown",
	// "None", "Parity", "Single-bit ECC", "Multi-bit ECC" or "CRC". Only the two ECC values and "None"
	// tell whether ECC is on. Parity only detects errors, and the others are rare or unknown.
	var errorCorrectionKnown, errorCorrectionOn bool
	switch errorCorrection {
	case "Single-bit ECC", "Multi-bit ECC":
		errorCorrectionKnown = true
		errorCorrectionOn = true
	case "None":
		errorCorrectionKnown = true
	}

	total, totalErr := strconv.Atoi(strings.TrimSuffix(totalWidth, " bits"))
	data, dataErr := strconv.Atoi(strings.TrimSuffix(dataWidth, " bits"))
	if totalErr != nil || dataErr != nil {
		// without the widths, error correction that is on still shows that ECC memory is installed.
		if errorCorrectionOn {
			return infrav1.ECCActive
		}
		return ""
	}

	// an ECC module has extra bits for the error correction, so its total width is larger than its data width.
	if total <= data {
		return infrav1.ECCNotInstalled
	}

	switch {
	case errorCorrectionOn:
		return infrav1.ECCActive
	case errorCorrectionKnown:
		return infrav1.ECCInactive
	default:
		return ""
	}
}

// parseInt32 parses value and checks the Minimum limit of its API field.
func parseInt32(key, value string, minimum int32) (int32, error) {
	n, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s %q: %w", key, value, err)
	}
	if int32(n) < minimum {
		return 0, fmt.Errorf("%s %d is below the minimum %d", key, n, minimum)
	}
	return int32(n), nil
}

// x86Level returns the x86-64 level of a CPU with these flags, from 1 to 4. It returns 0 when the
// CPU does not have the flag "lm" (long mode), because then the CPU does not run x86-64 code.
func x86Level(flags []string) int32 {
	if !slices.Contains(flags, "lm") {
		return 0
	}

	level := int32(1)
	for _, levelFlags := range x86LevelFlags {
		for _, flag := range levelFlags {
			if !slices.Contains(flags, flag) {
				return level
			}
		}
		level++
	}
	return level
}
