#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

PATH_BIN="/usr/local/bin"
MINIMUM_HELM_VERSION=v3.7.1



check_helm_installed() {
  # If helm is not available on the path, get it
  if ! [ -x "$(command -v helm)" ]; then
    echo 'helm not found, installing'
    install_helm
  fi
}


verify_helm_version() {

  local helm_version
  helm_version="$(helm version --template="{{ .Version }}")"
  if [[ "${MINIMUM_HELM_VERSION}" != $(echo -e "${MINIMUM_HELM_VERSION}\n${helm_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected helm version: ${helm_version}.
Requires ${MINIMUM_HELM_VERSION} or greater.
Please install ${MINIMUM_HELM_VERSION} or later.

EOF
    
    confirm "$@" && echo 'Installing Helm' && install_helm
  else
    cat <<EOF
Detected helm version: ${helm_version}.
Requires ${MINIMUM_HELM_VERSION} or greater.
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

install_helm() {
    if ! [ -d "${PATH_BIN}" ]; then
        mkdir -p "${PATH_BIN}"
    fi
    curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
    echo "Done"
}

check_helm_installed "$@"
verify_helm_version "$@"