#!/usr/bin/env bash

# Copyright 2014 The Kubernetes Authors.
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

if [ "$(uname)" = 'Darwin' ]; then
  readlinkf(){ perl -MCwd -e 'print Cwd::abs_path shift' "$1";}
else
  readlinkf(){ readlink -f "$1"; }
fi

# shellcheck disable=SC2128
SCRIPT_DIR="$(cd "$(dirname "$(readlinkf "$BASH_SOURCE")")" ; pwd)"

# We assume the link to the script ( {ensure,verify}-boilerplate.sh ) to be
# in a directory 2 levels down from the repo root, e.g. in
#   <root>/repo-infra/verify/verify-boilerplate.sh
# Alternatively, you can set the project root by setting the variable
# `REPO_ROOT`.
#
# shellcheck disable=SC2128
: "${REPO_ROOT:="$(cd "${SCRIPT_DIR}/.." ; pwd)"}"

boilerDir="${SCRIPT_DIR}/boilerplate/"
boiler="${boilerDir}/boilerplate.py"

verify() {
  # shellcheck disable=SC2207
  files_need_boilerplate=(
    $( "$boiler" --rootdir="$REPO_ROOT" --boilerplate-dir="$boilerDir" "$@")
  )

  # Run boilerplate check
  if [[ ${#files_need_boilerplate[@]} -gt 0 ]]; then
    for file in "${files_need_boilerplate[@]}"; do
      echo "Boilerplate header is wrong for: ${file}" >&2
    done

    return 1
  fi
}

ensure() {
  "$boiler" --rootdir="$REPO_ROOT" --boilerplate-dir="$boilerDir" --ensure "$@"
}

case "$0" in
  */ensure-boilerplate.sh)
    ensure "$@"
    ;;
  */verify-boilerplate.sh)
    verify "$@"
    ;;
  *)
    {
      echo "unknown command '$0'"
      echo ""
      echo "Call the script as either 'verify-boilerplate.sh' or 'ensure-boilerplate.sh'"
    } >&2

    exit 1
    ;;
esac
