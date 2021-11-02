#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# Ensure the go tool exists and is a viable version.
verify_go_version() {
  if [[ -z "$(command -v go)" ]]; then
    cat <<EOF
Can't find 'go' in PATH, please fix and retry.
See http://golang.org/doc/install for installation instructions.
EOF
    return 2
  fi

  local go_version
  IFS=" " read -ra go_version <<< "$(go version)"
  local minimum_go_version
  minimum_go_version=go1.16.9
  if [[ "${minimum_go_version}" != $(echo -e "${minimum_go_version}\n${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) && "${go_version[2]}" != "devel" ]]; then
    cat <<EOF
Detected go version: ${go_version[*]}.
This project requires ${minimum_go_version} or greater.
Please install ${minimum_go_version} or later.

EOF
    return 2
  fi
  if [[ "${minimum_go_version}" = $(echo -e "${minimum_go_version}\n${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) && "${go_version[2]}" != "devel" ]]; then
    cat <<EOF
Detected go version: ${go_version[*]}.
Nothing todo! You're up to date.

EOF
    return 0
  fi
}

verify_go_version

# Explicitly opt into go modules, even though we're inside a GOPATH directory
export GO111MODULE=on