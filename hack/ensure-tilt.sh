#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
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

REPO_ROOT=$(git rev-parse --show-toplevel)

install_tilt() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      cd ${REPO_ROOT}/hack/tools/bin && curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$MINIMUM_TILT_VERSION/tilt.$MINIMUM_TILT_VERSION.linux.x86_64.tar.gz | tar -xzv tilt && cd - 
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      cd ${REPO_ROOT}/hack/tools/bin && curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$MINIMUM_TILT_VERSION/tilt.$MINIMUM_TILT_VERSION.mac.x86_64.tar.gz | tar -xzv tilt && cd -
    else
      set +x
      echo "The installer does not work for your platform: $OSTYPE"
      exit 1
    fi
}


install_tilt "$@"