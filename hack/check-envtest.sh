#!/usr/bin/env bash
# Copyright 2025 The Kubernetes Authors.
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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

if [ "$#" -ne 2 ]; then
    echo "Error: Two arguments are required."
    exit 1
fi

SETUP_ENVTEST=$1
KUBEBUILDER_ENVTEST_KUBERNETES_VERSION=$2

if ! $SETUP_ENVTEST list | grep -q "$KUBEBUILDER_ENVTEST_KUBERNETES_VERSION"; then
    echo "$SETUP_ENVTEST is outdated. It does not support $KUBEBUILDER_ENVTEST_KUBERNETES_VERSION."
    echo "Remove $SETUP_ENVTEST and call make again."
    exit 1
fi

TOOLS_BIN_DIR=$(dirname "$SETUP_ENVTEST")

"$SETUP_ENVTEST" use --use-env --bin-dir "$TOOLS_BIN_DIR" -p path \
    "$KUBEBUILDER_ENVTEST_KUBERNETES_VERSION"
