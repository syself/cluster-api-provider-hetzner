#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
ROOT_PATH="$(cd "${SCRIPT_DIR}"/.. && pwd)"

VERSION="4.2.2"

MODE="check"

if [[ "$*" == "fix" ]]; then
  MODE="fix"
fi

if [[ "${OSTYPE}" == "linux"* ]]; then
  BINARY="buildifier"
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  BINARY="buildifier.mac"
fi

# create a temporary directory
TMP_DIR=$(mktemp -d)
OUT="${TMP_DIR}/out.log"

# cleanup on exit
cleanup() {
  ret=0
  if [[ -s "${OUT}" ]]; then
    echo "Found errors:"
    cat "${OUT}"
    echo ""
    echo
    echo 'Please review the above warnings. You can test via "./hack/verify-starlark.sh"'
    echo 'If the above warnings do not make sense, you can exempt this warning with a comment'
    echo ' (if your reviewer is okay with it).'
    echo 'In general please prefer to fix the error, we have already disabled specific lints.'
    echo "run make format-tiltfile to auto fix the errors"
    ret=1
  else
    echo 'Congratulations! All Tilt files are passing lint :-)'
  fi
  echo "Cleaning up..."
  rm -rf "${TMP_DIR}"
  exit ${ret}
}
trap cleanup EXIT

BUILDIFIER="${SCRIPT_DIR}/tools/bin/buildifier/${VERSION}/buildifier"

if [ ! -f "$BUILDIFIER" ]; then
  # install buildifier
  cd "${TMP_DIR}" || exit
  curl -L "https://github.com/bazelbuild/buildtools/releases/download/${VERSION}/${BINARY}" -o "${TMP_DIR}/buildifier"
  chmod +x "${TMP_DIR}/buildifier"
  cd "${ROOT_PATH}"
  mkdir -p "$(dirname "$0")/tools/bin/buildifier/${VERSION}"
  mv "${TMP_DIR}/buildifier" "$BUILDIFIER"
fi

echo "Running buildifier..."
cd "${ROOT_PATH}" || exit
IGNORE_FILES=$(find . -name "*.bazel" -o -name "*Tiltfile" | grep "third_party\|tilt_modules")
echo "Ignoring shellcheck on ${IGNORE_FILES}"
FILES=$(find . -name "*.bazel" -o -name "*Tiltfile" -not -path "./tilt_modules/*" -not -path "*third_party*")
while read -r file; do
    "${BUILDIFIER}" -mode=${MODE} "$file" >> "${OUT}" 2>&1 
done <<< "$FILES" 