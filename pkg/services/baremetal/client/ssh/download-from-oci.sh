#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
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

# This scripts gets copied from the controller into the rescue system
# of the bare-metal machine.

set -euo pipefail

image="${1:-}"
outfile="${2:-}"

function usage {
    echo "$0 image outfile."
    echo "  Download a machine image from a container registry"
    echo "  image: for example ghcr.io/foo/bar/my-machine-image:v9"
    echo "  outfile: Created file. Usualy with file extensions '.tgz'"
    echo "  If the oci registry needs a token, then the script uses OCI_REGISTRY_AUTH_TOKEN (if set)"
    echo "  Example of OCI_REGISTRY_AUTH_TOKEN: github:ghp_SN51...."
    echo
}
if [ -z "$outfile" ]; then
    usage
    exit 1
fi
OCI_REGISTRY_AUTH_TOKEN="${OCI_REGISTRY_AUTH_TOKEN:-}" # github:$GITHUB_TOKEN

# Extract registry
registry="${image%%/*}"

# Extract scope and tag
remainder="${image#*/}"
scope="${remainder%:*}"
tag="${remainder##*:}"

if [[ -z "$registry" || -z "$scope" || -z "$tag" ]]; then
    echo "failed to parse registry, scope and tag from image"
    echo "image=$image"
    echo "registry=$registry"
    echo "scope=$scope"
    echo "tag=$tag"
    exit 1
fi

function download_with_token {
    echo "download with token (OCI_REGISTRY_AUTH_TOKEN set)"
    if [[ "$OCI_REGISTRY_AUTH_TOKEN" != *:* ]]; then
        echo "OCI_REGISTRY_AUTH_TOKEN needs to contain a ':' (user:token)"
        exit 1
    fi

    token=$(curl -fsSL -u "$OCI_REGISTRY_AUTH_TOKEN" "https://${registry}/token?scope=repository:$scope:pull" | jq -r '.token')
    if [ -z "$token" ]; then
        echo "Failed to get token for container registry"
        exit 1
    fi

    echo "Login to $registry was successful"

    digest=$(curl -sSL -H "Authorization: Bearer $token" -H "Accept: application/vnd.oci.image.manifest.v1+json" \
        "https://${registry}/v2/${scope}/manifests/${tag}" | jq -r '.layers[0].digest')

    if [ -z "$digest" ]; then
        echo "Failed to get digest from container registry"
        exit 1
    fi

    echo "Start download of $image"
    curl -fsSL -H "Authorization: Bearer $token" \
        "https://${registry}/v2/${scope}/blobs/$digest" >"$outfile"
}

function download_without_token {
    echo "download without token (OCI_REGISTRY_AUTH_TOKEN empty)"
    digest=$(curl -sSL -H "Accept: application/vnd.oci.image.manifest.v1+json" \
        "https://${registry}/v2/${scope}/manifests/${tag}" | jq -r '.layers[0].digest')

    if [ -z "$digest" ]; then
        echo "Failed to get digest from container registry"
        exit 1
    fi

    echo "Start download of $image"
    curl -fsSL "https://${registry}/v2/${scope}/blobs/$digest" >"$outfile"
}

if [ -z "$OCI_REGISTRY_AUTH_TOKEN" ]; then
    download_without_token
else
    download_with_token
fi
