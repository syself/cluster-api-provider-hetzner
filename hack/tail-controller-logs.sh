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

ns=$(kubectl get deployments.apps -A | grep caph-controller-manager | cut -d' ' -f1)
pod=$(kubectl -n "$ns" get pods | grep caph-controller-manager | cut -d' ' -f1)

if [ -z "$pod" ]; then
    echo "failed to find caph-controller-manager pod"
    exit 1
fi

kubectl -n "$ns" logs "$pod" --tail 200 |
    ./hack/filter-caph-controller-manager-logs.py - |
    tail -n 10
