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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

image="${1:-}"
outfile="${2:-}"

function usage {
    echo "$0 image outfile"
    echo "  Download a machine image from a container registry"
    echo "  image: for example ghcr.io/foo/bar/my-machine-image:v9"
    echo "  outfile: Created file. Usually with file extensions '.tgz'"
    echo "  If the oci registry needs a token, then the script uses OCI_REGISTRY_AUTH_TOKEN (if set)"
    echo "  Example 1: of OCI_REGISTRY_AUTH_TOKEN: mygithubuser:mypassword"
    echo "  Example 2: of OCI_REGISTRY_AUTH_TOKEN: ghp_SN51...."
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

function get_token {
    echo "download with token (OCI_REGISTRY_AUTH_TOKEN set)"
    if [[ "$OCI_REGISTRY_AUTH_TOKEN" != *:* ]]; then
        echo "Using OCI_REGISTRY_AUTH_TOKEN directly (no colon in token)"
        token=$(echo "$OCI_REGISTRY_AUTH_TOKEN" | base64)
        return
    fi
    echo "OCI_REGISTRY_AUTH_TOKEN contains colon. Doing login first"
    token=$(curl -fsSL -u "$OCI_REGISTRY_AUTH_TOKEN" "https://${registry}/token?scope=repository:$scope:pull" | jq -r '.token')
    if [ -z "$token" ] || [ "$token" == null ]; then
        echo "Failed to get token for container registry"
        exit 1
    fi
    echo "Login to $registry was successful"
}

AUTH_ARGS=()
if [ -z "$OCI_REGISTRY_AUTH_TOKEN" ]; then
    echo "OCI_REGISTRY_AUTH_TOKEN is not set. Using no auth"
else
    token=""
    get_token
    if [ -z "$token" ]; then
        echo "failed to get token"
        exit 1
    fi
    AUTH_ARGS+=("--header")
    AUTH_ARGS+=("Authorization: Bearer $token")
fi
manifest=$(curl -sSL "${AUTH_ARGS[@]}" \
    -H "Accept: application/vnd.oci.image.manifest.v1+json" \
    "https://${registry}/v2/${scope}/manifests/${tag}")

if [ -z "$manifest" ] || [ "$manifest" == null ]; then
    echo "Failed to get manifest from container registry for image $image"
    exit 1
fi
digest=$(echo "$manifest" | jq -r '.layers[0].digest')

if [ -z "$digest" ] || [ "$digest" == null ]; then
    echo "Failed to get digest from container registry. Manifest: $manifest"
    exit 1
fi

expected_hash=$(echo "$manifest" | jq -r '.layers[0].digest' | cut -d':' -f2)
if [ -z "$expected_hash" ]; then
    echo "Could not get hash from manifest. Manifest: $manifest"
    exit 1
fi

echo "Start download of $image"
# with speed 5111000 bytes/sec (5MB/sec) and a 2 GByte image,
# it takes about 6 minutes to download the image.
# max-time 600 --> 10 minutes
# Usually the download is much fast: 40 MB/sec, which takes about 50 seconds.
curl -fsSL "${AUTH_ARGS[@]}" \
    --retry 5 --retry-delay 2 --retry-connrefused \
    --speed-limit 5111000 --speed-time 10 --max-time 600 \
    --continue-at - \
    --write-out "Downloaded %{size_download} bytes in %{time_total} seconds\n" \
    -o"$outfile" "https://${registry}/v2/${scope}/blobs/$digest"

hash=$(sha256sum "$outfile" | awk '{print $1}')
if [ -z "$hash" ]; then
    echo "Failed to calculate hash of downloaded file $outfile"
    exit 1
fi

if [ "$hash" != "$expected_hash" ]; then
    echo "Hash of downloaded file $outfile does not match expected hash"
    echo "Expected: $expected_hash"
    echo "Got: $hash"
    exit 1
fi

echo "Hash of downloaded file $outfile matches expected hash: $hash"
