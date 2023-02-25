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

set -o errexit
set -o nounset
set -o pipefail
set -x

semver_upgrade() {
IFS=. read -r version minor patch <<EOF
$VERSION
EOF

case "$1" in
patch) tag="$version.$minor.$((patch+1))"; ;;
major) tag="$((version+1)).0.0"; ;;
*)     tag="$version.$((minor+1)).0"; ;;
esac

echo $tag
}