#!/bin/bash

set -eu

curl \
    --fail-with-body \
    --retry 2 \
    --silent \
    -A "github.com/syself/cluster-api-provider-hetzner" \
    --header "Authorization: Bearer $TPS_TOKEN" \
    -X POST \
    https://tps.hc-integrations.de/
