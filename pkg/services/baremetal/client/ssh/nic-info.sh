#!/bin/bash
# Copyright 2025 The Kubernetes Authors.
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

for idir in $(echo /sys/class/net/* | sort); do
    iname=$(basename "$idir")

    # Skip loopback
    if [ "$iname" = "lo" ]; then
        continue
    fi

    # Skip interfaces that are not up
    if [ "$(cat "$idir/operstate")" != "up" ]; then
        continue
    fi

    MAC=$(cat "$idir/address")

    SPEED=$(ethtool "$iname" 2>/dev/null | awk '/Speed:/{print $2}' | sed 's/[^0-9]//g')

    # Grab the PCI bus info via ethtool, then get the model from lspci
    BUSINFO=$(ethtool -i "$iname" 2>/dev/null | awk '/bus-info:/{print $2}')

    if [ -z "$BUSINFO" ] || [ "$BUSINFO" = "N/A" ]; then
        MODEL="Unknown model"
    else
        MODEL=$(lspci -s "$BUSINFO" | cut -d ' ' -f3- | tr '"' "'")
    fi

    for ipv4 in $(ip -4 addr show dev "$iname" | awk '/inet /{print $2}' | cut -d'/' -f1); do
        echo "name=\"$iname\" model=\"$MODEL\" mac=\"$MAC\" ip=\"$ipv4\" speedMbps=\"$SPEED\""
    done
    for ipv6 in $(ip -6 addr show dev "$iname" | awk '/inet6 /{print $2}' | cut -d'/' -f1); do
        if [[ "$ipv6" == fe80:* ]]; then
            continue
        fi
        echo "name=\"$iname\" model=\"$MODEL\" mac=\"$MAC\" ip=\"$ipv6\" speedMbps=\"$SPEED\""
    done
done
