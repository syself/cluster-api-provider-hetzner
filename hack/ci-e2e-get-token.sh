#!/usr/bin/env bash
# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

if [[ "$TPS_TOKEN" == tps-11048c03556f* ]]; then
    echo "Error: Your TPS_TOKEN is outdated. Ask team mates for the new one."
    exit 1
fi

newtoken=$(curl -fsSL --retry 2 -A "github.com/syself/cluster-api-provider-hetzner" --header 'Authorization: Bearer '"$TPS_TOKEN"'' -X POST https://tps.hc-integrations.de)

if [ -z "${newtoken:-}" ]; then
    echo "Failed to get token from TPS"
    exit 1
fi

if [ -e .envrc ]; then
    echo "Updating .envrc"
    sed -i "s/^export HCLOUD_TOKEN=.*/export HCLOUD_TOKEN=$newtoken/" .envrc
    if type -P direnv >/dev/null; then
        direnv allow
    fi
fi
