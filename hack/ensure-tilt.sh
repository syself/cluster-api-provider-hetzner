#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

MINIMUM_TILT_VERSION=0.23.4


check_tilt_installed() {
  # If tilt is not available on the path, get it
  if ! [ -x "$(command -v tilt)" ]; then
    echo 'tilt not found, installing'
    install_tilt
  fi
}


verify_tilt_version() {

  local tilt_version
  tilt_version="$(tilt version | grep -Eo "([0-9]{1,}\.)+[0-9]{1,}")"
  if [[ "${MINIMUM_TILT_VERSION}" != $(echo -e "${MINIMUM_TILT_VERSION}\n${tilt_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected tilt version: ${tilt_version}.
Requires ${MINIMUM_TILT_VERSION} or greater.
Please install ${MINIMUM_TILT_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Tilt' && install_tilt
  else
    cat <<EOF
Detected tilt version: ${tilt_version}.
Requires ${MINIMUM_TILT_VERSION} or greater.
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

install_tilt() {
    if [[ "${OSTYPE}" == "linux"* ]]; then
      curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$MINIMUM_TILT_VERSION/tilt.$MINIMUM_TILT_VERSION.linux.x86_64.tar.gz | tar -xzv tilt
      copy_binary
    elif [[ "$OSTYPE" == "darwin"* ]]; then
      curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$MINIMUM_TILT_VERSION/tilt.$MINIMUM_TILT_VERSION.mac.x86_64.tar.gz | tar -xzv tilt
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
      mv tilt "$HOME/.local/bin/tilt"
  else
      echo "Installing Tilt to /usr/local/bin which is write protected"
      echo "If you'd prefer to install Tilt without sudo permissions, add \$HOME/.local/bin to your \$PATH and rerun the installer"
      sudo mv tilt /usr/local/bin/tilt
  fi
  echo "Installation Finished"
}

check_tilt_installed "$@"
verify_tilt_version "$@"