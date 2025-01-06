#!/bin/bash

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

    for ipv4 in $(ip -4 addr show dev "$iname" | awk '/inet /{print $2}'); do
        echo "name=\"$iname\" model=\"$MODEL\" mac=\"$MAC\" ip=\"$ipv4\" speedMbps=\"$SPEED\""
    done
    for ipv6 in $(ip -6 addr show dev "$iname" | awk '/inet6 /{print $2}'); do
        echo "name=\"$iname\" model=\"$MODEL\" mac=\"$MAC\" ip=\"$ipv6\" speedMbps=\"$SPEED\""
    done
done
