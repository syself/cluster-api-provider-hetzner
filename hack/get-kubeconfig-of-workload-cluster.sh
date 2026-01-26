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

CLUSTER_NAME=${CLUSTER_NAME:-}
CLUSTER_NAMESPACE=${CLUSTER_NAMESPACE:-}

if [ -z "$CLUSTER_NAME" ]; then
    # Pick the first cluster found if none is provided explicitly.
    CLUSTER_NAME=$(kubectl get clusters -A -o jsonpath='{.items[0].metadata.name}')
    CLUSTER_NAMESPACE=$(kubectl get clusters -A -o jsonpath='{.items[0].metadata.namespace}')
fi

if [ -z "$CLUSTER_NAME" ]; then
    echo "env var CLUSTER_NAME is missing and no clusters found. Failed to get kubeconfig of workload cluster"
    exit 1
fi

# Default to the Cluster namespace when not provided.
CLUSTER_NAMESPACE=${CLUSTER_NAMESPACE:-default}

kubeconfig=".workload-cluster-kubeconfig.yaml"
new_content="$(kubectl -n "$CLUSTER_NAMESPACE" get secrets "${CLUSTER_NAME}-kubeconfig" -ojsonpath='{.data.value}' | base64 -d)"

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
