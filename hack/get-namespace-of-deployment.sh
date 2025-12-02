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

if [[ $# -ne 1 ]] || [[ "$1" == -* ]]; then
    echo "Usage: $0 <deployment-name>" >&2
    exit 1
fi

dep="$1"

# Find the namespace (must be exactly one)
ns_candidates="$(kubectl get deploy -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.namespace}{"\n"}{end}' |
    awk -v d="$dep" '$1==d{print $2}')"

ns_count="$(printf '%s\n' "$ns_candidates" | sed '/^$/d' | wc -l | tr -d ' ')"
if [ "$ns_count" -eq 0 ]; then
    echo "ERROR: Deployment '$dep' not found in any namespace." >&2
    exit 1
elif [ "$ns_count" -gt 1 ]; then
    echo "ERROR: Deployment '$dep' found in multiple namespaces:" >&2
    printf '%s\n' "$ns_candidates" >&2
    exit 1
fi
printf '%s\n' "$ns_candidates" | head -n1
