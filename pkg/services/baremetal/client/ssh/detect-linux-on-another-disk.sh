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

set -euo pipefail

trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR

function usage() {
    echo "$0 wwn1 [wwn2 ...]"
    echo "    Check if there is a Linux partition, but skip all WWNs given as arguments"
    echo "    Background: If we provision a disk, then there must not be a Linux OS on an other partition"
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
    if ! lsblk -l -oWWN | grep -qP '^'${wwn}'$'; then
        echo "$wwn is not a WWN of this machine"
        echo
        usage
        exit 3
    fi
done
fail=0
while read name wwn type parttype; do
    if [[ " $* " == *" $wwn "* ]]; then
        #echo "ok: skipping $name $wwn, since it was an argument to the script."
        continue
    fi
    root_directory_content=$(grub-fstest /dev/$name ls / 2>/dev/null || true | tr ' ' '\n' | sort | tr '\n' ' ')
    if [[ $root_directory_content =~ .*boot/.*etc/.* ]]; then
        echo "FAIL: $name $wwn partitionType=$parttype looks like a Linux root partition on another disk."
        fail=1
        continue
    fi
    if [[ $root_directory_content =~ .*initrd.*vmlinuz.* ]]; then
        echo "FAIL: $name $wwn partitionType=$parttype looks like a Linux /boot partition on another disk."
        fail=1
        continue
    fi
    #echo "ok: $name $wwn $parttype, does not look like root Linux partition."
done < <(lsblk -r -oNAME,WWN,TYPE,PARTTYPENAME | grep -v NAME | grep -i part)
if [ $fail -eq 1 ]; then
    exit 1
fi
echo "Looks good. No Linux root partition on other devices"
