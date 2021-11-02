#!/usr/bin/env bash

set -o errexit
set -o pipefail

K8S_VERSION=v1.22.0

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}" || exit 1

source "${REPO_ROOT}/hack/ci-e2e-lib.sh"

# Make sure the tools binaries are on the path.
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"

echo ""
echo "Cluster initialising... Please hold on"
echo ""
ctlptl_kind-cluster-with-registry caph ${K8S_VERSION}

echo ""
echo ""
echo ""
echo "Cluster is ready - you can now tilt up!"