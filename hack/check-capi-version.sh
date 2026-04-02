#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
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
#
# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

# Get the directory of the script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

# Extract the desired CAPI version from go.mod
DESIRED_VERSION=$(grep -E 'sigs.k8s.io/cluster-api v' go.mod | grep -v '/test' | awk '{print $2}')

if [[ -z "${DESIRED_VERSION}" ]]; then
    echo "❌ Could not find cluster-api version in go.mod"
    exit 1
fi

echo "✓ Desired CAPI version from go.mod: ${DESIRED_VERSION}"
echo ""

# Find all non-documentation files with cluster-api/clusterctl version mismatches
# Docs are excluded as they can contain example output that doesn't need to be in sync
if git ls-files | grep -v vendor | grep -v '\.md$' | xargs grep -nH -E '(cluster-api|clusterctl|capi_version)' 2>/dev/null | grep -E 'v1\.[0-9]+\.[0-9]+' | grep -v "${DESIRED_VERSION}"; then
    echo ""
    echo "❌ Version mismatches found! Expected: ${DESIRED_VERSION}"
    echo "Please update the mismatched files to use the version from go.mod"
    exit 1
fi

DESIRED_SERIES="${DESIRED_VERSION#v}"
DESIRED_SERIES="${DESIRED_SERIES%.*}"
DESIRED_MAJOR="${DESIRED_SERIES%%.*}"
DESIRED_MINOR="${DESIRED_SERIES##*.}"
METADATA_FILE="test/e2e/data/shared/v1beta1/metadata.yaml"

if ! awk \
    -v desired_major="${DESIRED_MAJOR}" \
    -v desired_minor="${DESIRED_MINOR}" \
    '
        /^[[:space:]]*-[[:space:]]*major:[[:space:]]*/ {
            major = $0
            sub(/.*major:[[:space:]]*/, "", major)
            next
        }
        /^[[:space:]]*minor:[[:space:]]*/ {
            minor = $0
            sub(/.*minor:[[:space:]]*/, "", minor)
            if (major == desired_major && minor == desired_minor) {
                found = 1
            }
        }
        END { exit(found ? 0 : 1) }
    ' "${METADATA_FILE}"; then
    echo "❌ Missing releaseSeries entry for CAPI ${DESIRED_MAJOR}.${DESIRED_MINOR} in ${METADATA_FILE}"
    echo "Please update the shared e2e metadata to include the major/minor series from go.mod"
    exit 1
fi

echo "✅ All CAPI versions are in sync with go.mod (${DESIRED_VERSION})"
