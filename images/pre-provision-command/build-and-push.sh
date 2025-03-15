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

# This script builds and pushed the container image for the init-container.
# See README.md for more information.

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

DIR="$(dirname "$0")"

docker build -t ghcr.io/syself/caph-staging:pre-provision-command "$DIR"

docker push ghcr.io/syself/caph-staging:pre-provision-command
