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

set -o errexit
set -o pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}" || exit 1

# Make sure the tools binaries are on the path.
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"

export REGISTRY=quay.io/syself
export IMAGE_NAME=cluster-api-provider-hetzner
export TAG=e2e
export PULL_POLICY=IfNotPresent

# Builds CAPH images, if missing
if [[ "$(docker images -q "$REGISTRY/$IMAGE_NAME:$TAG" 2> /dev/null)" == "" ]]; then
  echo "+ Building CAPH images"
  make e2e-image
else
  echo "+ CAPH images already present in the system, skipping make"
fi

# Configure e2e tests
export GINKGO_NODES=1
export GINKGO_NOCOLOR=true
export GINKGO_SKIP="Conformance"
export GINKGO_ARGS="--failFast" # Other ginkgo args that need to be appended to the command.
export E2E_CONF_FILE="${REPO_ROOT}/test/e2e/config/hetzner.yaml"
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"
export SKIP_RESOURCE_CLEANUP=false
export USE_EXISTING_CLUSTER=false

# We need to export the HCLOUD_TOKEN as a environment variable
SSH_KEY_NAME=caph-e2e-$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c 12 ; echo '')
SSH_KEY_PATH=/tmp/${SSH_KEY_NAME}
create_ssh_key() {
    echo "generating new ssh key"
    ssh-keygen -t ed25519 -f ${SSH_KEY_PATH} -N '' 2>/dev/null <<< y >/dev/null
    echo "importing ssh key "
    hcloud ssh-key create --name ${SSH_KEY_NAME} --public-key-from-file ${SSH_KEY_PATH}.pub
}

remove_ssh_key() {
    local ssh_fingerprint=$1
    echo "removing ssh key"
    hcloud ssh-key delete ${SSH_KEY_NAME}
    rm -f ${SSH_KEY_PATH}

    ${REPO_ROOT}/hack/log/redact.sh || true
}

create_ssh_key
trap 'remove_ssh_key ${SSH_KEY_NAME}' EXIT
export HCLOUD_SSH_KEY=${SSH_KEY_NAME}

# Generate manifests


# Run e2e tests
mkdir -p "$ARTIFACTS"
echo "+ run tests!"
make -C test/e2e/ run

test_status="${?}"