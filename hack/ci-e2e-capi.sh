#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
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

trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

./hack/ensure-env-variables.sh HCLOUD_WORKER_MACHINE_TYPE HCLOUD_CONTROL_PLANE_MACHINE_TYPE HCLOUD_REGION

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
cd "${REPO_ROOT}" || exit 1

# Make sure the tools binaries are on the path.
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"

# We need to export the HCLOUD_TOKEN as a environment variable
SSH_KEY_NAME=caph-e2e-$(
    LC_CTYPE=C dd if=/dev/urandom bs=1 count=100 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | head -c 12
    echo ''
)
export SSH_KEY_PATH=/tmp/${SSH_KEY_NAME}
export SSH_KEY_NAME=${SSH_KEY_NAME}

create_ssh_key() {
    echo "generating new ssh key"
    ssh-keygen -t ed25519 -f "$SSH_KEY_PATH" -N '' 2>/dev/null <<<y >/dev/null
    echo "importing ssh key "
    hcloud ssh-key create --name "$SSH_KEY_NAME" --public-key-from-file "$SSH_KEY_PATH".pub
}

remove_ssh_key() {
    echo "removing ssh key"
    hcloud ssh-key delete "$SSH_KEY_NAME"
    rm -f "$SSH_KEY_PATH"
    "$REPO_ROOT"/hack/log/redact.sh || true
}

if ! output=$(curl -fsS -H "Authorization: Bearer $HCLOUD_TOKEN" 'https://api.hetzner.cloud/v1/ssh_keys' 2>&1); then
    echo "HCLOUD_TOKEN is invalid: $output"
    exit 1
fi

create_ssh_key "$SSH_KEY_NAME" "$SSH_KEY_PATH"
trap 'remove_ssh_key ${SSH_KEY_NAME}' EXIT

mkdir -p "$ARTIFACTS"
echo "+ run tests!"

IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/syself}"

if [[ -z "${CAPH_CONTAINER_TAG:-}" ]]; then
    echo
    echo "Error: Missing CAPH_CONTAINER_TAG environment variable"
    echo "This is the caph container image tag for the image."
    echo "For PRs this is pr-NNNN"
    echo "Use the following command to set the environment variable:"
    echo "  gh pr view --json number --jq .number"
    echo "Then: export CAPH_CONTAINER_TAG=pr-NNNN"
    echo
    exit 1
fi

make -C test/e2e/ run GINKGO_NODES="${GINKGO_NODES}" GINKGO_FOCUS="${GINKGO_FOKUS}"
