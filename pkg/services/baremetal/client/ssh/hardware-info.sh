#!/bin/bash
# Copyright 2026 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Prints the board, firmware, CPU, memory, watchdog and PCI device details of the host as
# key=value lines. The script does not stop when one value cannot be read. The command that
# failed writes to stderr, and the controller logs it.

set -uo pipefail

# the value of a field in the first processor block of /proc/cpuinfo, for example "6" for "cpu family".
cpuinfo() {
    awk -F: -v key="$1" '{ k = $1; sub(/[ \t]+$/, "", k) } k == key { sub(/^[ \t]+/, "", $2); print $2; exit }' /proc/cpuinfo
}

echo "board_vendor=$(cat /sys/class/dmi/id/board_vendor)"
echo "board_name=$(cat /sys/class/dmi/id/board_name)"
echo "board_version=$(cat /sys/class/dmi/id/board_version)"
echo "firmware_vendor=$(cat /sys/class/dmi/id/bios_vendor)"
echo "firmware_version=$(cat /sys/class/dmi/id/bios_version)"

# the kernel creates /sys/firmware/efi only when the firmware booted in UEFI mode.
if [ -d /sys/firmware/efi ]; then
    echo "boot_mode=UEFI"
else
    echo "boot_mode=Legacy"
fi

echo "cpu_family=$(cpuinfo "cpu family")"
echo "cpu_model_id=$(cpuinfo "model")"
echo "cpu_stepping=$(cpuinfo "stepping")"
echo "cpu_microcode=$(cpuinfo "microcode")"
echo "cpu_flags=$(cpuinfo "flags")"

# the type of the first installed module. Empty slots report "Size: No Module Installed" and "Type: Unknown".
echo "memory_type=$(dmidecode -t 17 | awk -F': ' '/^[ \t]+Size:/ { size = $2 } /^[ \t]+Type:/ && size !~ /No Module/ { print $2; exit }')"
echo "memory_ecc=$(dmidecode -t 16 | awk -F': ' '/Error Correction Type:/ { print $2; exit }')"

watchdog_driver=""
if [ -e /sys/class/watchdog/watchdog0/device/driver ]; then
    watchdog_driver=$(basename "$(readlink /sys/class/watchdog/watchdog0/device/driver)")
fi
echo "watchdog_driver=${watchdog_driver}"

# one line for each storage (class 0x01), network (0x02) and display (0x03) device:
# pci_device=<address> <vendor ID> <device ID> <class> <driver>
for device in /sys/bus/pci/devices/*; do
    class=$(cat "$device/class")
    case "$class" in
    0x01* | 0x02* | 0x03*) ;;
    *) continue ;;
    esac
    driver=""
    if [ -e "$device/driver" ]; then
        driver=$(basename "$(readlink "$device/driver")")
    fi
    echo "pci_device=$(basename "$device") $(cat "$device/vendor") $(cat "$device/device") ${class} ${driver}"
done
