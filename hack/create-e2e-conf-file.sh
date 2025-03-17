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

trap 'echo "ERROR: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

./hack/ensure-env-variables.sh CAPH_LATEST_VERSION ENVSUBST E2E_CONF_FILE_SOURCE E2E_CONF_FILE TAG

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

make release-manifests
export MANIFEST_PATH="../../../out"

echo "# Created from $E2E_CONF_FILE_SOURCE by $0" >"$E2E_CONF_FILE"
$ENVSUBST <"$E2E_CONF_FILE_SOURCE" >>"$E2E_CONF_FILE"
