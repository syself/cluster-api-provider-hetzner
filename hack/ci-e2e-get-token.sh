#!/bin/bash

set -eu

newtoken=$(curl --fail-with-body --retry 2 --silent -A "github.com/syself/cluster-api-provider-hetzner" --header 'Authorization: Bearer '"$TPS_TOKEN"'' -X POST https://tps.hc-integrations.de)

if [ -e .envrc ]; then
    echo "Updating .envrc"
    sed -i "s/^export HCLOUD_TOKEN=.*/export HCLOUD_TOKEN=$newtoken/" .envrc
    if type -P direnv >/dev/null; then
        direnv allow
    fi
fi
