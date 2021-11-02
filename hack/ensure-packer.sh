#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

MINIMUM_PACKER_VERSION=1.7.7



check_packer_installed() {
  # If packer is not available on the path, get it
  if ! [ -x "$(command -v packer)" ]; then
    echo 'packer not found, installing'
    install_packer
  fi
}


verify_packer_version() {

  local packer_version
  packer_version="$(packer version | sed 's/[^ ]* *//' | sed 's/v//')"
  if [[ "${MINIMUM_PACKER_VERSION}" != $(echo -e "${MINIMUM_PACKER_VERSION}\n${packer_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected packer version: v${packer_version}.
Requires v${MINIMUM_PACKER_VERSION} or greater.
Please install v${MINIMUM_PACKER_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Packer' && install_packer
  else
    cat <<EOF
Detected packer version: v${packer_version}.
Requires v${MINIMUM_PACKER_VERSION} or greater.
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

install_packer() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -sLo "packer.zip" https://releases.hashicorp.com/packer/${MINIMUM_PACKER_VERSION}/packer_${MINIMUM_PACKER_VERSION}_linux_amd64.zip && unzip packer.zip -d packer-bin && mv packer-bin/packer . && rm -rf packer.zip packer-bin
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -sLo "packer.zip" https://releases.hashicorp.com/packer/${MINIMUM_PACKER_VERSION}/packer_${MINIMUM_PACKER_VERSION}_darwin_amd64.zip && unzip packer.zip -d packer-bin && mv packer-bin/packer . && rm -rf packer.zip packer-bin
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
      mv packer "$HOME/.local/bin/packer"
      chmod +x "$HOME/.local/bin/packer"
  else
      echo "Installing Packer to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Packer without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv packer /usr/local/bin/packer
      chmod +x "/usr/local/bin/packer"
  fi
  echo "Installation Finished"
}

check_packer_installed "$@"
verify_packer_version "$@"
