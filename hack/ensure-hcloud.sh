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

check_hcloud_installed() {
  # If hcloud is not available on the path, get it
  if ! [ -x "$(command -v hcloud)" ]; then
    echo 'hcloud not found, installing'
    install_hcloud
  fi
}


verify_hcloud_version() {

  local hcloud_version
  hcloud_version="$(hcloud version | sed 's/[^ ]* *//' )"
  if [[ "${MINIMUM_HCLOUD_VERSION}" != $(echo -e "${MINIMUM_HCLOUD_VERSION}\n${hcloud_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected hcloud version: ${hcloud_version}.
Requires ${MINIMUM_HCLOUD_VERSION} or greater.
Please install ${MINIMUM_HCLOUD_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing HCLOUD' && install_hcloud
  else
    cat <<EOF
Detected hcloud version: ${hcloud_version}.
Requires ${MINIMUM_HCLOUD_VERSION} or greater.
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

install_hcloud() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -fsSL https://github.com/hetznercloud/cli/releases/download/v${MINIMUM_HCLOUD_VERSION}/hcloud-linux-amd64.tar.gz | tar -xzv hcloud
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -fsSL https://github.com/hetznercloud/cli/releases/download/v${MINIMUM_HCLOUD_VERSION}/hcloud-macos-amd64.tar.gz | tar -xzv hcloud
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
      mv hcloud "$HOME/.local/bin/hcloud"
      chmod +x "$HOME/.local/bin/hcloud"
  else
      echo "Installing HCLOUD to /usr/local/bin which is write protected"
      echo "If you'd prefer to install HCLOUD without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv hcloud /usr/local/bin/hcloud
      chmod +x "/usr/local/bin/hcloud"
  fi
  echo "Installation Finished"
}

check_hcloud_installed "$@"
verify_hcloud_version "$@"