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

set -euo pipefail

if [ -z "${CLUSTER_NAME:-}" ]; then
    echo "env var CLUSTER_NAME is missing. Failed to get kubeconfig of workload cluster"
    exit 1
fi

if [ -z "${KUBECONFIG:-}" ]; then
    echo "env var KUBECONFIG (for mgt-cluster) is missing. Failed to get kubeconfig of workload cluster"
    exit 1
fi

if [[ ! -e $KUBECONFIG ]]; then
    echo "KUBECONFIG=$KUBECONFIG file does not exist!  Failed to get kubeconfig of workload cluster"
    exit 1
fi

kubeconfig=".workload-cluster-kubeconfig.yaml"
if ! new_content="$(kubectl get secrets "${CLUSTER_NAME}-kubeconfig" -ojsonpath='{.data.value}' 2>/dev/null | base64 -d)"; then
    echo "error: Failed to get kubeconfig of wl-cluster"
    exit 1
fi

if [ -z "$new_content" ]; then
    echo "failed to get kubeconfig of workload cluster"
    exit 1
fi

# If we create this fail again and again (via `make watch`), then there is a race-condition
# This can lead to makefile targets fail, because the file is empty for a fraction of a second.
if [ -s "$kubeconfig" ]; then
    old_content="$(cat $kubeconfig)"
    if [ "$new_content" == "$old_content" ]; then
        # Correct kubeconfig already exits, nothing to do.
        exit 0
    fi
fi
echo "$new_content" >"$kubeconfig"
chmod a=,u=rw $kubeconfig
