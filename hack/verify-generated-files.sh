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
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
    echo
    echo "Pre Start of verify-generated-faile.sh"
    echo "Error: Git repository is not clean. Please commit, stash, or remove your changes and untracked files before proceeding."
    git status
    exit 1
fi

(
    export PATH="$(git rev-parse --show-toplevel)/hack/tools/bin:$PATH"
    make kubectl
    cd test/e2e
    HCLOUD_TOKEN=dummy_hcloud_token
    HETZNER_SSH_PUB=$(echo dummy-hetzner-ssh-pub | base64)
    HETZNER_SSH_PRIV=$(echo dummy-hetzner-ssh-priv | base64)
    SSH_KEY_NAME="dummy-ssh-key-name"
    HETZNER_ROBOT_USER="dummy-hetzner-robot-user"
    HETZNER_ROBOT_PASSWORD="dummy-hetzner-robot-password"
    export HCLOUD_TOKEN HETZNER_SSH_PUB HETZNER_SSH_PRIV SSH_KEY_NAME HETZNER_ROBOT_USER HETZNER_ROBOT_PASSWORD
    make e2e-cilium-templates
    make e2e-ccm-templates
    make cluster-templates
)

make generate

if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
    echo "After generated files got re-generated:"
    echo "Error: Git repository is not clean. Please commit, stash, or remove your changes and untracked files before proceeding."
    git status
    echo
    echo "-------------------------"
    git diff
    echo
    echo "git hash: $(git rev-parse HEAD)"
    exit 1
fi

echo "OK: No changes in git repo after re-creating generated files".
