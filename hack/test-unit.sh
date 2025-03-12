#!/usr/bin/env bash
# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

if ! ./hack/tools/bin/setup-envtest list | grep -q "$KUBEBUILDER_ENVTEST_KUBERNETES_VERSION"; then
    echo "./hack/tools/bin/setup-envtest is outdated. It does not support $KUBEBUILDER_ENVTEST_KUBERNETES_VERSION."
    echo "Remove ./hack/tools/bin/setup-envtest and call make again."
    exit 1
fi

KUBEBUILDER_ASSETS=$(./hack/tools/bin/setup-envtest use --use-env \
    --bin-dir "$PWD/hack/tools/bin" -p path \
    "$KUBEBUILDER_ENVTEST_KUBERNETES_VERSION")

export KUBEBUILDER_ASSETS

mkdir -p .coverage

hack/tools/bin/gotestsum --junitfile=.coverage/junit.xml --format testname -- -covermode=atomic -coverprofile=.coverage/cover.out -p=4 -timeout 5m ./controllers/... ./pkg/... ./api/...
