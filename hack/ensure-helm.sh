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

PATH_BIN="/usr/local/bin"

check_helm_installed() {
  # If helm is not available on the path, get it
  if ! [ -x "$(command -v helm)" ]; then
    echo 'helm not found, installing'
    install_helm
  fi
}

verify_helm_version() {

  local helm_version
  helm_version="$(helm version --template="{{ "{{" }} .Version {{ "}}" }}")"
  if [[ "v${MINIMUM_HELM_VERSION}" != $(echo -e "v${MINIMUM_HELM_VERSION}\n${helm_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected helm version: ${helm_version}.
Requires ${MINIMUM_HELM_VERSION} or greater.
Please install ${MINIMUM_HELM_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Helm' && install_helm
  else
    cat <<EOF
Detected helm version: ${helm_version}.
Requires ${MINIMUM_HELM_VERSION} or greater.
Nothing to do!

EOF
  fi
}

confirm() {
    # call with a prompt string or use a default
    echo "${1:-Do you want to install? [y/N]}"
    read -r -p "" response
    case "$response" in
        [yY][eE][sS]|[yY]) 
            true
            ;;
        *)
            false
            return 2
            ;;
    esac
}

install_helm() {
    if ! [ -d "${PATH_BIN}" ]; then
        mkdir -p "${PATH_BIN}"
    fi
    curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
    echo "Done"
}

check_helm_installed "$@"
verify_helm_version "$@"