#!/usr/bin/env bash
# Copyright 2025 The Kubernetes Authors.
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

# This scripts updates the caph deployment in a mgt-cluster. Be sure to be connected to
# a development cluster.
# It takes the current code, creates a new image and updates the deployment.
# By default the image will be uploaded to:
# ghcr.io/syself/caph-staging:dev-$USER-YOUR_GIT_BRANCH
# You can change the image path with the --image-path flag, if you want to use a different registry.

# Usually it is better to write a test, than to use this script.

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

if [[ $(kubectl config current-context) == *oidc@* ]]; then
    echo "found oidc@ in the current kubectl context. It is likely that you are connected"
    echo "to the wrong cluster"
    exit 1
fi

image_path="ghcr.io/syself"

while [[ "$#" -gt 0 ]]; do
    case $1 in
    --image-path)
        image_path="$2"
        shift
        ;;
    *)
        echo "Unknown parameter passed: $1"
        exit 1
        ;;
    esac
    shift
done

# remove trailing slash
image_path="${image_path%/}"

if ! kubectl cluster-info >/dev/null; then
    echo
    echo "No kubernetes cluster found."
    echo "You can use alm to create a mgt-cluster"
    echo "docs: https://github.com/syself/autopilot-lifecycle-manager"
    exit 1
fi

current_context=$(kubectl config current-context)
if ! echo "$current_context" | grep -P '.*-admin@.*-mgt-cluster'; then
    echo "The script refuses to update because the current context is: $current_context"
    echo "Expecting something like foo-mgt-cluster-admin@foo-mgt-cluster with 'foo' being a short version of your name"
    exit 1
fi

branch=$(git branch --show-current)

# No branch name was found. Try to get a git-tag.
if [ "$branch" == "" ]; then
    branch=$(git describe --tags --exact-match 2>/dev/null || echo "")
fi

if [ "$branch" == "" ]; then
    echo "failed to find git branch/tag name"
    exit 1
fi

# Build tag for container image.
tag="dev-$USER-$branch"
tag="$(echo -n "$tag" | tr -c 'a-zA-Z0-9_.-' '-')"

image="$image_path/caph-staging:$tag"

# Fail if cluster-api-operator is running
if kubectl get pods -A | grep -q cluster-api-operator; then
    echo "Error: cluster-api-operator is running!"
    echo "Changes to caph deployment and its CRDs would be reverted."
    echo "Hint: Scale down replicas of the cluster-api-operator deployment."
    exit 1
fi

# run in background
{
    make generate-manifests
    kustomize build config/crd | kubectl apply -f -
} &
pid_generate=$!

# run in background
{
    docker build -f images/caph/Dockerfile -t "$image" . --progress=plain
    docker push "$image"
} &
pid_docker_push=$!

# wait for both processes and check their exit codes
if ! wait $pid_generate; then
    echo "Error: generate-manifests/kustomize failed"
    exit 1
fi

if ! wait $pid_docker_push; then
    echo "Error: docker build/push failed"
    exit 1
fi

# Find namespace of caph deployment.
ns=$(kubectl get deployments.apps -A | { grep caph-controller || true; } | cut -d' ' -f1)
if [[ -z $ns ]]; then
    echo "failed to get namespace for caph-controller"
    exit 1
fi

# Scale deployment to 1.
kubectl scale --replicas=1 -n "$ns" deployment/caph-controller-manager

kubectl set image -n "$ns" deployment/caph-controller-manager manager="$image"

# Set imagePullPolicy to "Always", so that a new image (with same name/tag) gets pulled.
kubectl patch deployment -n "$ns" -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Always"}]' --type='json' caph-controller-manager

# If you update the image again, there might be no change the deployment spec.
# Force a rollout:
kubectl rollout restart -n "$ns" deployment caph-controller-manager

trap "echo 'Interrupted! Exiting...'; exit 1" SIGINT

while ! kubectl rollout status deployment --timeout=3s -n "$ns" caph-controller-manager; do
    echo "Rollout failed"
    kubectl events -n "$ns" | grep caph-controller-manager | tail -n 5
    echo
    echo
done
