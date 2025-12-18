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

./hack/ensure-env-variables.sh CAPH_LATEST_VERSION E2E_CONF_FILE_SOURCE E2E_CONF_FILE CAPH_CONTAINER_TAG

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

make release-manifests

# `make release-manifests` changes local files (caph image). Restore them,
# so they do not get committed accidentally.
git restore config

echo "# Created from $E2E_CONF_FILE_SOURCE by $0" >"$E2E_CONF_FILE"
$CLUSTERCLT generate yaml <"$E2E_CONF_FILE_SOURCE" >>"$E2E_CONF_FILE"
