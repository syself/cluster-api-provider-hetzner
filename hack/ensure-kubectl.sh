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

check_kubectl_installed() {
  # If kubectl is not available on the path, get it
  if ! [ -x "$(command -v kubectl)" ]; then
    echo 'kubectl not found, installing'
    install_kubectl
  fi
}

verify_kubectl_version() {

  local kubectl_version
  kubectl_version="$(kubectl version --client --short | grep -Eo "([0-9]{1,}\.)+[0-9]{1,}")"
  if [[ "${MINIMUM_KUBECTL_VERSION}" != $(echo -e "${MINIMUM_KUBECTL_VERSION}\n${kubectl_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected kubectl version: ${kubectl_version}.
Requires ${MINIMUM_KUBECTL_VERSION} or greater.
Please install ${MINIMUM_KUBECTL_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Kubectl' && install_kubectl
  else
    cat <<EOF
Detected kubectl version: ${kubectl_version}.
Requires ${MINIMUM_KUBECTL_VERSION} or greater.
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

install_kubectl() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -sLo "kubectl" https://dl.k8s.io/release/v${MINIMUM_KUBECTL_VERSION}/bin/linux/amd64/kubectl
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -sLo "kubectl" https://dl.k8s.io/release/v${MINIMUM_KUBECTL_VERSION}/bin/darwin/amd64/kubectl
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
      mv kubectl "$HOME/.local/bin/kubectl"
      chmod +x "$HOME/.local/bin/kubectl"
  else
      echo "Installing Kubectl to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Kubectl without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv kubectl /usr/local/bin/kubectl
      chmod +x "/usr/local/bin/kubectl"
  fi
  echo "Installation Finished"
}

check_kubectl_installed "$@"
verify_kubectl_version "$@"


# TODO Krew Plugin support and kubectl plugins: argocd rollout