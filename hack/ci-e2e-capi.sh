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

./hack/ensure-env-variables.sh HCLOUD_WORKER_MACHINE_TYPE HCLOUD_CONTROL_PLANE_MACHINE_TYPE HCLOUD_REGION SSH_KEY_NAME HETZNER_SSH_PUB HETZNER_SSH_PRIV

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
cd "${REPO_ROOT}" || exit 1

# Make sure the tools binaries are on the path.
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"

if ! output=$(curl -fsS -H "Authorization: Bearer $HCLOUD_TOKEN" 'https://api.hetzner.cloud/v1/ssh_keys' 2>&1); then
    echo "HCLOUD_TOKEN is invalid: $output"
    exit 1
fi

# Create ssh-key if it does not exist yet
if ! hcloud ssh-key list | grep -qF "$SSH_KEY_NAME"; then
    echo "info: Creating ssh-key in hcloud"
    echo "$HETZNER_SSH_PUB" | hcloud ssh-key create --name "$SSH_KEY_NAME" --public-key-from-file -
fi

echo "info: You can connect to the machines with ssh-key $SSH_KEY_NAME"

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
