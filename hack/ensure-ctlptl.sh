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

check_ctlptl_installed() {
  # If ctlptl is not available on the path, get it
  if ! [ -x "$(command -v ctlptl)" ]; then
    echo 'ctlptl not found, installing'
    install_ctlptl
  fi
}


verify_ctlptl_version() {

  local ctlptl_version
  ctlptl_version="$(ctlptl version | grep -Eo "([0-9]{1,}\.)+[0-9]{1,}")"
  if [[ "${MINIMUM_CTLPTL_VERSION}" != $(echo -e "${MINIMUM_CTLPTL_VERSION}\n${ctlptl_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected ctlptl version: ${ctlptl_version}.
Requires ${MINIMUM_CTLPTL_VERSION} or greater.
Please install ${MINIMUM_CTLPTL_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing CTLPTL' && install_ctlptl
  else
    cat <<EOF
Detected ctlptl version: ${ctlptl_version}.
Requires ${MINIMUM_CTLPTL_VERSION} or greater.
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

install_ctlptl() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v${MINIMUM_CTLPTL_VERSION}/ctlptl.${MINIMUM_CTLPTL_VERSION}.linux.x86_64.tar.gz | tar -xzv ctlptl
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v${MINIMUM_CTLPTL_VERSION}/ctlptl.${MINIMUM_CTLPTL_VERSION}.mac.x86_64.tar.gz | tar -xzv ctlptl
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
      mv ctlptl "$HOME/.local/bin/ctlptl"
      chmod +x "$HOME/.local/bin/ctlptl"
  else
      echo "Installing CTLPTL to /usr/local/bin which is write protected"
      echo "If you'd prefer to install CTLPTL without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv ctlptl /usr/local/bin/ctlptl
      chmod +x "/usr/local/bin/ctlptl"
  fi
  echo "Installation Finished"
}

check_ctlptl_installed "$@"
verify_ctlptl_version "$@"