#!/usr/bin/env bash

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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

if ! ./hack/tools/bin/setup-envtest list | grep -q "$KUBEBUILDER_ENVTEST_KUBERNETES_VERSION"; then
    echo "./hack/tools/bin/setup-envtest is outdated. It does not support $KUBEBUILDER_ENVTEST_KUBERNETES_VERSION."
    echo "Remove ./hack/tools/bin/setup-envtest and call make again."
    exit 1
fi

KUBEBUILDER_ASSETS=$(./hack/tools/bin/setup-envtest use --use-env \
    --bin-dir "$PWD/hack/tools/bin" -p path \
    "$KUBEBUILDER_ENVTEST_KUBERNETES_VERSION")

export KUBEBUILDER_ASSETS

mkdir -p .coverage

hack/tools/bin/gotestsum \
    --jsonfile=.reports/go-test-output.json \
    --junitfile=.coverage/junit.xml \
    --format testname -- \
    -covermode=atomic -coverprofile=.coverage/cover.out -p=4 -timeout 5m \
    ./controllers/... ./pkg/... ./api/...
