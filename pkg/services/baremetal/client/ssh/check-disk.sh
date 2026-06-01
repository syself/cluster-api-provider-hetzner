#!/bin/bash

# Copyright 2024 The Kubernetes Authors.
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

trap 'echo "ERROR: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

function usage() {
    echo "$0 wwn1 [wwn2 ...]"
    echo "    Check given disks."
    echo "    Exit 0: Disks look good."
    echo "    Exit 1: Disks seem faulty."
    echo "    Exit 3: Some other error (like invalid WWN)"
    echo "Existing WWNs:"
    lsblk -oNAME,WWN | grep -vi loop || true
}

if [ $# -eq 0 ]; then
    echo "Error: No WWN was provided."
    echo
    usage
    exit 3
fi

# Show usage, if any argument starts with a dash.
for arg in "$@"; do
    if [[ "$arg" == -* ]]; then
        usage
        exit 3
    fi
done

wait_for_wwns() {
    local deadline=$((SECONDS + 30))
    local known_wwns

    while [ "$SECONDS" -lt "$deadline" ]; do
        known_wwns=$(lsblk --nodeps --noheadings -o WWN 2>/dev/null || true)
        local all_found=1
        for wwn in "$@"; do
            if ! grep -qFx -- "$wwn" <<<"$known_wwns"; then
                all_found=0
                break
            fi
        done
        if [ "$all_found" -eq 1 ]; then
            return 0
        fi
        sleep 1
    done

    echo "INFO: Timed out waiting for requested WWNs to appear in lsblk. Continuing with current device state."
}

# Wait only for the requested disks instead of the full udev queue.
wait_for_wwns "$@"

install_smartmontools() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        case "$ID" in
        debian | ubuntu)
            sudo apt-get update -qq
            sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq -o Dpkg::Progress-Fancy="0" smartmontools |
                { grep -vP '^(NEEDRESTART|Selecting previously unselected|.Reading database|Preparing to unpack|Unpacking|Setting up|Processing)' || true; }
            ;;
        centos | rhel | fedora)
            sudo yum install -y smartmontools
            ;;
        opensuse | sles)
            sudo zypper install --non-interactive smartmontools
            ;;
        arch | manjaro)
            sudo pacman -Sy --noconfirm smartmontools
            ;;
        *)
            echo "Unsupported distribution: $ID"
            exit 1
            ;;
        esac
    else
        echo "Cannot detect the operating system."
        exit 1
    fi
}

install_fio() {
    if [[ -f /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        case "$ID" in
        debian | ubuntu)
            sudo apt-get update -qq
            sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq -o Dpkg::Progress-Fancy="0" fio |
                { grep -vP '^(NEEDRESTART|Selecting previously unselected|.Reading database|Preparing to unpack|Unpacking|Setting up|Processing)' || true; }
            ;;
        centos | rhel | fedora)
            sudo yum install -y fio
            ;;
        opensuse | sles)
            sudo zypper install --non-interactive fio
            ;;
        arch | manjaro)
            sudo pacman -Sy --noconfirm fio
            ;;
        *)
            echo "Unsupported distribution: $ID"
            exit 1
            ;;
        esac
    else
        echo "Cannot detect the operating system."
        exit 1
    fi
}

# In the rescue system smartctl is always available. This is just needed if the
# script gets executed by hand (outside caph)
if ! type smartctl >/dev/null 2>&1; then
    echo "INFO: smartctl not installed yet. If possible, please provide smartmontools in your machine image."
    install_smartmontools
fi

if ! type fio >/dev/null 2>&1; then
    echo "INFO: fio not installed yet. If possible, please provide fio in your machine image."
    install_fio
fi

result=$(mktemp)
trap 'rm -f "$result"' EXIT

fio_errors=""

# Iterate over all input arguments
for wwn in "$@"; do
    if ! lsblk -l -oWWN | grep -qFx "${wwn}"; then
        echo "$wwn is not a WWN of this machine"
        echo
        usage
        exit 3
    fi
    device=$(lsblk -oNAME,WWN,TYPE | grep disk | grep "$wwn" | cut -d' ' -f1)
    if [ -z "$device" ]; then
        echo "Failed to find device for WWN $wwn"
        exit 3
    fi
    echo "Checking WWN=$wwn device=$device"
    { smartctl -H "/dev/$device" || true; } | { grep -vP '^(smartctl \d+\.\d+.*|Copyright|=+ START OF)' || true; } |
        { grep -v '^$' || true; } |
        { sed "s#^#$wwn (/dev/$device): #" || true; } >>"$result"

    rota=$(lsblk -d -n -o ROTA "/dev/$device")
    if [ "$rota" -eq 1 ]; then
        # Rotational (HDD): healthy 7200 RPM drives do 100-200 MiB/s sequential
        min_bw_kib=40000
        disk_type="HDD"
    else
        # Non-rotational (SSD/NVMe): SATA SSDs do 400+ MiB/s, NVMe much more
        min_bw_kib=160000
        disk_type="SSD/NVMe"
    fi

    echo "Running fio sequential read check on /dev/$device ($disk_type, threshold: $((min_bw_kib / 1024)) MiB/s)..."
    fio_out=$(fio --name=check --rw=read --bs=128k --filename="/dev/$device" --direct=1 \
        --runtime=3 --time_based --output-format=terse 2>/dev/null)

    # terse v3 field 7 (1-indexed): read bandwidth in KiB/s
    bw_kib=$(awk -F';' 'NR==1{print $7}' <<<"$fio_out")
    if ! [[ "$bw_kib" =~ ^[0-9]+$ ]]; then
        echo "ERROR: Could not parse fio output for /dev/$device"
        exit 3
    fi

    bw_mib=$((bw_kib / 1024))
    if [ "$bw_kib" -lt "$min_bw_kib" ]; then
        msg="$wwn (/dev/$device, $disk_type): read bandwidth ${bw_mib} MiB/s is below threshold $((min_bw_kib / 1024)) MiB/s"
        echo "FAIL: $msg"
        fio_errors="${fio_errors}"$'\n'"${msg}"
    else
        echo "OK: $wwn (/dev/$device, $disk_type): read bandwidth ${bw_mib} MiB/s"
    fi
done

errors=$(grep -v PASSED "$result" || true)
if [ -n "$errors" ]; then
    #some lines don't contain "PASSED". There was an error.
    echo "check-disk failed!"
    echo "$errors"
    exit 1
fi

if [ -n "$fio_errors" ]; then
    echo "check-disk failed (low disk performance)!"
    echo "$fio_errors"
    exit 1
fi

echo "check-disk passed. Provided WWNs look healthy."
echo
cat "$result"
exit 0
