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

check_helmfile_installed() {
  # If helmfile is not available on the path, get it
  if ! [ -x "$(command -v helmfile)" ]; then
    echo 'helmfile not found, installing'
    install_helmfile
  fi
}


verify_helmfile_version() {

  local helmfile_version
  helmfile_version="v$(helmfile version | grep -Eo "([0-9]{1,}\.)+[0-9]{1,}")"
  if [[ "${MINIMUM_HELMFILE_VERSION}" != $(echo -e "${MINIMUM_HELMFILE_VERSION}\n${helmfile_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected helmfile version: ${helmfile_version}.
Requires ${MINIMUM_HELMFILE_VERSION} or greater.
Please install ${MINIMUM_HELMFILE_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Helmfile' && install_helmfile
  else
    cat <<EOF
Detected helmfile version: ${helmfile_version}.
Requires ${MINIMUM_HELMFILE_VERSION} or greater.
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

install_helmfile() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -fsSL "helmfile" https://github.com/helmfile/helmfile/releases/download/v${MINIMUM_HELMFILE_VERSION}/helmfile_${MINIMUM_HELMFILE_VERSION}_linux_amd64.tar.gz | tar -xzv helmfile
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -fsSL "helmfile" https://github.com/helmfile/helmfile/releases/download/v${MINIMUM_HELMFILE_VERSION}/helmfile_${MINIMUM_HELMFILE_VERSION}_darwin_amd64.tar.gz | tar -xzv helmfile
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
      mv helmfile "$HOME/.local/bin/helmfile"
      chmod +x "$HOME/.local/bin/helmfile"
  else
      echo "Installing Helmfile to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Helmfile without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv helmfile /usr/local/bin/helmfile
      chmod +x "/usr/local/bin/helmfile"
  fi
  echo "Installation Finished"
}


check_helmfile_installed "$@"
verify_helmfile_version "$@"