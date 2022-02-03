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

check_clusterctl_installed() {
  # If clusterctl is not available on the path, get it
  if ! [ -x "$(command -v clusterctl)" ]; then
    echo 'clusterctl not found, installing'
    install_clusterctl
  fi
}


verify_clusterctl_version() {

  local clusterctl_version
  clusterctl_version="$(clusterctl version -o short | sed 's/v//')"
  if [[ "${MINIMUM_CLUSTERCTL_VERSION}" != $(echo -e "${MINIMUM_CLUSTERCTL_VERSION}\n${clusterctl_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected clusterctl version: v${clusterctl_version}.
Requires v${MINIMUM_CLUSTERCTL_VERSION} or greater.
Please install v${MINIMUM_CLUSTERCTL_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Clusterctl' && install_clusterctl
  else
    cat <<EOF
Detected clusterctl version: v${clusterctl_version}.
Requires v${MINIMUM_CLUSTERCTL_VERSION} or greater.
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

install_clusterctl() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -sLo "clusterctl" https://github.com/kubernetes-sigs/cluster-api/releases/download/v${MINIMUM_CLUSTERCTL_VERSION}/clusterctl-linux-amd64
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -sLo "clusterctl" https://github.com/kubernetes-sigs/cluster-api/releases/download/v${MINIMUM_CLUSTERCTL_VERSION}/clusterctl-darwin-amd64
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
      mv clusterctl "$HOME/.local/bin/clusterctl"
      chmod +x "$HOME/.local/bin/clusterctl"
  else
      echo "Installing Clusterctl to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Clusterctl without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv clusterctl /usr/local/bin/clusterctl
      chmod +x "/usr/local/bin/clusterctl"
  fi
  echo "Installation Finished"
}

check_clusterctl_installed "$@"
verify_clusterctl_version "$@"
