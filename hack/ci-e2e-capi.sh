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

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
cd "${REPO_ROOT}" || exit 1

# Make sure the tools binaries are on the path.
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"

# shellcheck source=../hack/ci-e2e-sshkeys.sh
source "${REPO_ROOT}/hack/ci-e2e-sshkeys.sh"

# We need to export the HCLOUD_TOKEN as a environment variable
SSH_KEY_NAME=caph-e2e-$(
    LC_CTYPE=C dd if=/dev/urandom bs=1 count=100 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | head -c 12
    echo ''
)
export SSH_KEY_PATH=/tmp/${SSH_KEY_NAME}
export SSH_KEY_NAME=${SSH_KEY_NAME}
create_ssh_key ${SSH_KEY_NAME} ${SSH_KEY_PATH}
trap 'remove_ssh_key ${SSH_KEY_NAME}' EXIT

mkdir -p "$ARTIFACTS"
echo "+ run tests!"

IMAGE_PREFIX="${IMAGE_PREFIX:-ghcr.io/syself}"

if [[ -z "${TAG:-}" ]]; then
    echo
    echo "Error: Missing TAG environment variable"
    echo "This is the caph container image tag for the image."
    echo "For PRs this is pr-NNNN"
    echo "Use the following command to set the environment variable:"
    echo "  gh pr view --json number --jq .number"
    echo "Then: export TAG=pr-NNNN"
    echo
    exit 1
fi

make set-manifest-image "MANIFEST_IMG=${IMAGE_PREFIX}/caph-staging" "MANIFEST_TAG=${TAG}"
make set-manifest-pull-policy PULL_POLICY=IfNotPresent

echo "***************************** E2E_CONF_FILE $E2E_CONF_FILE *****************************"
cat "$E2E_CONF_FILE"
echo "***************************** E2E_CONF_FILE end *****************************"

make -C test/e2e/ run GINKGO_NODES="${GINKGO_NODES}" GINKGO_FOCUS="${GINKGO_FOKUS}"
