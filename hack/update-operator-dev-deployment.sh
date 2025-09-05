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

trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

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
if [ "$branch" == "" ]; then
    echo "failed to get branch name"
    exit 1
fi

tag="dev-$USER-$branch"
tag="$(echo -n "$tag" | tr -c 'a-zA-Z0-9_.-' '-')"

image="$image_path/caph-staging:$tag"

echo "Building image: $image"

docker build -f images/caph/Dockerfile -t "$image" .

docker push "$image"

make generate-manifests

kustomize build config/crd | kubectl apply -f -

kubectl scale --replicas=1 -n mgt-system deployment/caph-controller-manager

kubectl set image -n mgt-system deployment/caph-controller-manager manager="$image"

kubectl patch deployment -n mgt-system -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Always"}]' --type='json' caph-controller-manager

kubectl rollout restart -n mgt-system deployment caph-controller-manager

trap "echo 'Interrupted! Exiting...'; exit 1" SIGINT

while ! kubectl rollout status deployment --timeout=3s -n mgt-system caph-controller-manager; do
    echo "Rollout failed"
    kubectl events -n mgt-system | grep caph-controller-manager | tail -n 5
    echo
    echo
done
