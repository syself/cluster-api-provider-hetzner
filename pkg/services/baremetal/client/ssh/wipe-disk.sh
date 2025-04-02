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
    echo "    Wipe all filesystem, raid or partition-table signatures from the specified disks."
    echo "    ATTENTION! THIS DELETES ALL DATA ON THE GIVEN DISK!"
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

    lsblk --json --paths "/dev/$device" | grep -Po '/dev/md\w+' | sort -u | while read -r md; do
        echo "INFO: Stopping mdraid $md for $wwn (/dev/$device)"
        mdadm --stop "$md"
    done

    echo "INFO: Calling wipefs for $wwn (/dev/$device)"
    wipefs -af "/dev/$device"
done
