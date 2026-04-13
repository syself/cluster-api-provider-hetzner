#!/usr/bin/env bash

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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

dep="caph-controller-manager"

hack_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ns=$("$hack_dir"/get-namespace-of-deployment.sh $dep)
pod=$("$hack_dir"/get-leading-pod.sh $dep "$ns")
kubectl -n "$ns" logs "$pod" --tail 200 |
    "$hack_dir"/filter-caph-controller-manager-logs.py - |
    tail -n 10
