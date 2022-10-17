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

check_kind_installed() {
  # If kind is not available on the path, get it
  if ! [ -x "$(command -v kind)" ]; then
    echo 'kind not found, installing'
    install_kind
  fi
}


verify_kind_version() {

  local kind_version
  kind_version="$(kind version -q)"
  if [[ "${MINIMUM_KIND_VERSION}" != $(echo -e "${MINIMUM_KIND_VERSION}\n${kind_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected kind version: ${kind_version}.
Requires ${MINIMUM_KIND_VERSION} or greater.
Please install ${MINIMUM_KIND_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Kind' && install_kind
  else
    cat <<EOF
Detected kind version: ${kind_version}.
Requires ${MINIMUM_KIND_VERSION} or greater.
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

install_kind() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -sLo "kind" https://github.com/kubernetes-sigs/kind/releases/download/v${MINIMUM_KIND_VERSION}/kind-linux-amd64
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -sLo "kind" https://github.com/kubernetes-sigs/kind/releases/download/v${MINIMUM_KIND_VERSION}/kind-darwin-amd64
      copy_binary
    else
      set +x
      echo "The installer does not work for your platform: $OSTYPE"
      exit 1
    fi

}

function copy_binary() {
  if [[ ":$PATH:" == *":$HOME/.local/bin:"* ]]; then
      if [ ! -d "$HOME/.local/bin" ]; then
        mkdir -p "$HOME/.local/bin"
      fi
      mv kind "$HOME/.local/bin/kind"
      chmod +x "$HOME/.local/bin/kind"
  else
      echo "Installing Kind to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Kind without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv kind /usr/local/bin/kind
      chmod +x "/usr/local/bin/kind"
  fi
  echo "Installation Finished"
}

check_kind_installed "$@"
verify_kind_version "$@"