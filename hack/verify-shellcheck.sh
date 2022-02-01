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
VERSION="v0.8.0"

# disabled lints
disabled=(
  # this lint disallows non-constant source, which we use extensively without
  # any known bugs
  1090
  # this lint prefers command -v to which, they are not the same
  2230

  1091
)

OS="unknown"
if [[ "${OSTYPE}" == "linux"* ]]; then
  OS="linux"
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  OS="darwin"
fi

# comma separate for passing to shellcheck
join_by() {
  local IFS="$1";
  shift;
  echo "$*";
}
# shellcheck source=./hack/utils.sh
source "$(dirname "$0")/lib/utils.sh"
ROOT_PATH=$(get_root_path)

# create a temporary directory
TMP_DIR=$(mktemp -d)
OUT="${TMP_DIR}/out.log"


# cleanup on exit
cleanup() {
  ret=0
  if [[ -s "${OUT}" ]]; then
    echo "Found errors:"
    cat "${OUT}"
    echo
    echo 'Please review the above warnings. You can test via "./hack/verify-shellcheck.sh"'
    echo 'If the above warnings do not make sense, you can exempt this warning with a comment'
    echo ' (if your reviewer is okay with it).'
    echo 'In general please prefer to fix the error, we have already disabled specific lints.'
    echo 'See: https://github.com/koalaman/shellcheck/wiki/Ignore#ignoring-one-specific-instance-in-a-file'
    echo
    ret=1
  else
    echo 'Congratulations! All shell files are passing lint :-)'
  fi
  echo "Cleaning up..."
  rm -rf "${TMP_DIR}"
  exit ${ret}
}
trap cleanup EXIT

SHELLCHECK_DISABLED="$(join_by , "${disabled[@]}")"
readonly SHELLCHECK_DISABLED

SHELLCHECK="./$(dirname "$0")/tools/bin/shellcheck/${VERSION}/shellcheck"

if [ ! -f "$SHELLCHECK" ]; then
  # install buildifier
  cd "${TMP_DIR}" || exit
  DOWNLOAD_FILE="shellcheck-${VERSION}.${OS}.x86_64.tar.xz"
  curl -L "https://github.com/koalaman/shellcheck/releases/download/${VERSION}/${DOWNLOAD_FILE}" -o "${TMP_DIR}/shellcheck.tar.xz"
  tar xf "${TMP_DIR}/shellcheck.tar.xz"
  cd "${ROOT_PATH}"
  mkdir -p "$(dirname "$0")/tools/bin/shellcheck/${VERSION}"
  mv "${TMP_DIR}/shellcheck-${VERSION}/shellcheck" "$SHELLCHECK"
fi

echo "Running shellcheck..."
cd "${ROOT_PATH}" || exit
IGNORE_FILES=$(find . -name "*.sh" | grep "third_party\|tilt_modules|node_modules")
echo "Ignoring shellcheck on ${IGNORE_FILES}"
FILES=$(find . -name "*.sh" -not -path "./tilt_modules/*" -not -path "*third_party*" -not -path "*node_modules*")
while read -r file; do
    "$SHELLCHECK" -x  "--exclude=${SHELLCHECK_DISABLED}" "--color=auto" "$file" >> "${OUT}" 2>&1 
done <<< "$FILES" 
