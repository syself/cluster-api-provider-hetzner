#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
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

# This script is executed in the Update-Bot container.
# It checks if the Dockerfile for the build container has changed.
# If so, it uses the version of the main branch as the basis for creating a new image tag. 
# The script also checks if the image tag for the build image exists in the main branch. 
# We assume that only one branch and one PR will be created for the changes in the build container. 
# Therefore, we can update this branch as many times as we want and reuse the next available tag after the one from the main branch.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(realpath $(dirname "${BASH_SOURCE[0]}")/..)
cd "${REPO_ROOT}" || exit 1

source "${REPO_ROOT}/hack/semver-upgrade.sh"

if git diff --exit-code images/builder/Dockerfile images/builder/build.sh > /dev/null; then
exit 0
fi

if [ "${CI:-false}" = true ] ; then
echo $BUILD_IMAGE_TOKEN | docker login ghcr.io -u $BUILD_IMAGE_USER --password-stdin
fi

export VERSION=$(git fetch --quiet origin main && git show origin/main:Makefile | grep "BUILDER_IMAGE_VERSION :=" | sed 's/.*BUILDER_IMAGE_VERSION := //' | sed 's/\s.*$//' )
export NEW_VERSION=$(semver_upgrade patch ${VERSION})

if docker manifest inspect ghcr.io/syself/caph-builder:${VERSION} > /dev/null ; echo $?; then
  
  sed -i -e "/^BUILDER_IMAGE_VERSION /s/:=.*$/:= ${NEW_VERSION}/" Makefile
  for FILE in ${REPO_ROOT}/.github/workflows/*; do
    if grep "image: ghcr.io/syself/caph-builder" $FILE
    then
      sed -i -e "/image: ghcr\.io\/syself\/caph-builder:/s/:.*$/: ghcr\.io\/syself\/caph-builder:${NEW_VERSION}/" $FILE
    fi
  done
  docker build -t ghcr.io/syself/caph-builder:${NEW_VERSION}  ./images/builder
  docker push ghcr.io/syself/caph-builder:${NEW_VERSION}
else
  exit 1
fi