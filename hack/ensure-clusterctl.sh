#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

MINIMUM_CLUSTERCTL_VERSION=1.0.0



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
