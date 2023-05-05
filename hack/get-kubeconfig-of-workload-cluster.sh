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

cluster_name=$(jq -r .kustomize_substitutions.CLUSTER_NAME tilt-settings.json)
kubeconfig=".workload-cluster-kubeconfig.yaml"
kubectl get secrets "$cluster_name-kubeconfig" -ojsonpath='{.data.value}' | base64 -d > "$kubeconfig"

if [ ! -s "$kubeconfig" ]; then
    echo "failed to get kubeconfig of workload cluster"
    exit 1
fi

chmod a=,u=rw $kubeconfig
