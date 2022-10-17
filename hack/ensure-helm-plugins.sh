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



helm_plugins_to_install () {
    verify_helm_plugin_version "helm-git" "https://github.com/aslafy-z/helm-git" "${HELM_GIT_VERSION}"
    verify_helm_plugin_version "diff" "https://github.com/databus23/helm-diff" "${HELM_DIFF_VERSION}"
}

check_helm_installed() {
  # If helm is not available on the path, get it
  if ! [ -x "$(command -v helm)" ]; then
    echo 'helm not found!'
  fi
}

check_helm_plugin_installed() {
  if ! [[ "$(helm plugin list | grep "$1" | cut -f 1 | xargs )" = "$1" ]]; then
    echo "helm plugin $1 not found"
    install_helm_plugin "$@"
  fi
}

verify_helm_plugin_version() {

  check_helm_plugin_installed "$@"

  local helm_plugin_version
  local helm_minimum_plugin_version

  helm_plugin_version="$(helm plugin list | grep "$1" | cut -f 2 )"
  helm_minimum_plugin_version="$3"

  if [[ "${helm_minimum_plugin_version}" != $(echo -e "${helm_minimum_plugin_version}\n${helm_plugin_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected helm plugin version of $1: ${helm_plugin_version}
Requires ${helm_minimum_plugin_version} or greater.
Please install ${helm_minimum_plugin_version} or later.

EOF
    
    confirm && echo "Installing Helm Plugin $1" && update_helm_plugin "$@"
  else
    cat <<EOF
Detected helm pluginversion: ${helm_plugin_version}
Requires ${helm_minimum_plugin_version} or greater.
Nothing to do!

EOF
  fi
}

confirm() {
    # call with a prompt string or use a default
    echo "Do you want to install? [y/N]}"
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

install_helm_plugin() {
    echo "installing $1"
    helm plugin install "$2" --version "$3"
    echo "Done"
}

update_helm_plugin() {
    echo "installing $1"
    helm plugin remove "$1"
    helm plugin install "$2" --version "$3"
    echo "Done"
}

check_helm_installed "$@"
helm_plugins_to_install "$@"