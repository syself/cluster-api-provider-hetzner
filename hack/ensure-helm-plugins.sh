#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}" || exit 1

helm_plugins_to_install () {
    verify_helm_plugin_version "helm-git" "https://github.com/aslafy-z/helm-git" "0.10.0"
    verify_helm_plugin_version "diff" "https://github.com/databus23/helm-diff" "3.1.3"
}

check_helm_installed() {
  # If helm is not available on the path, get it
  if ! [ -x "$(command -v helm)" ]; then
    echo 'helm not found, installing'
    source "${REPO_ROOT}/hack/helm.sh"
  fi
}

check_helm_plugin_installed() {
  if ! [[ "$(helm plugin list | grep "$1" | cut -f 1 | xargs )" = "$1" ]]; then
    echo "helm plugin $1 not found"
    install_helm_plugin "$@"
  fi
}

verify_helm_plugin_version() {

  check_helm_plugin_installed "$@"

  local helm_plugin_version
  local helm_minimum_plugin_version

  helm_plugin_version="$(helm plugin list | grep "$1" | cut -f 2 )"
  helm_minimum_plugin_version="$3"

  if [[ "${helm_minimum_plugin_version}" != $(echo -e "${helm_minimum_plugin_version}\n${helm_plugin_version}" | sort -s -t. -k 1,1n -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected helm plugin version of $1: ${helm_plugin_version}.
Requires ${helm_minimum_plugin_version} or greater.
Please install ${helm_minimum_plugin_version} or later.

EOF
    
    confirm && echo "Installing Helm Plugin $1" && update_helm_plugin "$@"
  else
    cat <<EOF
Detected helm version: ${helm_plugin_version}.
Requires ${helm_minimum_plugin_version} or greater.
Nothing to do!

EOF
  fi
}

confirm() {
    # call with a prompt string or use a default
    echo "Do you want to install? [y/N]}"
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

install_helm_plugin() {
    echo "installing $1"
    helm plugin install "$2" --version "$3"
    echo "Done"
}

update_helm_plugin() {
    echo "installing $1"
    helm plugin update "$2" --version "$3"
    echo "Done"
}

check_helm_installed "$@"
helm_plugins_to_install "$@"