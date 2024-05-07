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

# lsblk from util-linux 2.34 (Ubuntu 20.04) does not know column PARTTYPENAME

trap 'echo "ERROR: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

function usage() {
    echo "$0 wwn1 [wwn2 ...]"
    echo "    Check if there is a Linux partition, but skip all WWNs given as arguments"
    echo "    Background: If we provision a disk, then there must not be a Linux OS on another partition"
    echo "    otherwise it is likely that the old OS gets booted, and not the new OS."
    echo "    Exit 0: If there is no Linux installation found."
    echo "    Exit 1: There is a Linux on a different disk.".
    echo "    Exit 3: Unexpected error."
    echo "Existing WWNs:"
    lsblk -oNAME,WWN | grep -vi loop || true
}

if [ $# -eq 0 ]; then
    echo "Error: No WWN was provided."
    echo
    usage
    exit 3
fi

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
    pv=$(pvs | grep -F "$device" || true)
    if [ -z "$pv" ]; then
        continue
    fi
    echo "fail: There is a physical volume on $wwn ($device)"
    echo "To ensure no data gets deleted, provisioning on disks containing physical volumes is not supported".
    echo "Please use different rootDeviceHints or clean the disk (for example with 'wipefs')"
    exit 1
done
fail=0

lines=$(lsblk -r -oNAME,WWN,TYPE)

while read -r name wwn; do
    if [[ " $* " == *" $wwn "* ]]; then
        #echo "ok: skipping $name $wwn, since it was an argument to the script."
        continue
    fi
    root_directory_content=$(grub-fstest "/dev/$name" ls / 2>/dev/null || true | tr ' ' '\n' | sort | tr '\n' ' ')
    if [[ $root_directory_content =~ .*boot/.*etc/.* ]]; then
        echo "FAIL: $name $wwn looks like a Linux root partition on another disk."
        fail=1
        continue
    fi
    if [[ $root_directory_content =~ .*initrd.*vmlinuz.* ]]; then
        echo "FAIL: $name $wwn looks like a Linux /boot partition on another disk."
        fail=1
        continue
    fi
    if echo "$root_directory_content" | grep -Pqi '.*\bEFI\b.*'; then # -i is needed, because grub-fstest prints "efi", but mounted it is "EFI".
        echo "FAIL: $name $wwn looks like a Linux /boot/efi partition on another disk."
        fail=1
        continue
    fi
    ## echo "ok: $name $wwn $parttype, does not look like root Linux partition."
done < <(echo "$lines" | grep -v NAME | grep -i part)
if [ $fail -eq 1 ]; then
    exit 1
fi

# Check mdraids: If an existing mdraid spans the root-devices and non-root-devices, then fail.

# Write all WWNs which will contain the new OS into a file.
wwn_file=$(mktemp)
for wwn in "$@"; do
    echo "$wwn" >>"$wwn_file"
done
sort --unique -o "$wwn_file" "$wwn_file"

md_file=$(mktemp)
shopt -s nullglob
for mdraid in /dev/md?*; do
    rm -f "$md_file"
    device=$(basename "$mdraid")
    for dev_of_mdraid in /sys/block/"$device"/md/dev-*; do
        dev_of_mdraid=$(echo "$dev_of_mdraid" | cut -d- -f2)
        wwn=$(udevadm info --query=property "--name=$dev_of_mdraid" | grep ID_WWN | cut -d= -f2)
        if [ -z "$wwn" ]; then
            echo "<<<<<<<<<<<<<<<<<<<<"
            udevadm info --query=property "--name=$dev_of_mdraid"
            echo ">>>>>>>>>>>>>>>>>>>>"
            echo "failed to get WWN of $dev_of_mdraid"
            exit 1
        fi
        echo "$wwn" >>"$md_file"
    done
    if [ ! -s "$md_file" ]; then
        echo "failed to find devices of $mdraid"
        exit 1
    fi
    sort --unique -o "$md_file" "$md_file"
    if cmp --silent "$md_file" "$wwn_file"; then
        echo "mdraid $mdraid is ok. It will contain the new operating system."
        continue
    fi

    # Print only lines present in both files.
    intersection=$(comm -12 <(sort "$md_file") <(sort "$wwn_file"))
    if [ -z "$intersection" ]; then
        echo "mdraid $mdraid is ok. It contains no devices which will be used for the operation system."
        continue
    fi
    echo "fail: mdraid $mdraid contains devices which should be used for the new operating system."
    echo "      And $mdraid contains devices which will not be used for the new operating system."
    echo "      Cluster API Provider Hetzner won't provision the machine like this, since"
    echo "      after running 'installimage' the old raid could be started instead of the new OS."
    echo "--- Content of /proc/mdstat:"
    cat /proc/mdstat
    echo "-------"
    echo "The new OS should be installed on these WWNs: $*"
    lsblk -oNAME,WWN | grep -vi loop || true
    echo "-------"
    echo "failed."
    exit 1
done

rm -f "$md_file" "$wwn_file"

echo "Looks good. No Linux root partition on another devices"
