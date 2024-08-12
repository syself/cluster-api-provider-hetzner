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
if ! type smartctl >/dev/null 2>&1; then
    echo "INFO: smartctl not installed yet. If possible, please provide smartmontools in your machine image."
    install_smartmontools
fi

result=$(mktemp)
trap 'rm -f "$result"' EXIT

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
    smartctl -H "/dev/$device" | { grep -vP '^(smartctl \d+\.\d+.*|Copyright|=+ START OF SMART DATA SECTION.*)' || true; } |
        { grep -v '^$' || true; } |
        sed "s#^#$wwn (/dev/$device): #" >>"$result"
done
errors=$(grep -v PASSED "$result" || true)
if [ -n "$errors" ]; then
    #some lines don't contain "PASSED". There was an error.
    echo "check-disk failed!"
    echo "$errors"
    exit 1
fi
cat "$result"
exit 0
