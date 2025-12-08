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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

ROBOT=
while [[ $# -gt 0 ]]; do
    case "$1" in
    --robot)
        ROBOT=1
        shift
        ;;
    *)
        break
        ;;
    esac
done

if [[ $# -ne 2 ]]; then
    echo "Usage: $0 [--robot] API_VERSION NAME"
    exit 1
fi

API_VERSION="$1"
case "$API_VERSION" in
v1beta1 | v1beta2) ;;
*)
    echo "Error: API_VERSION must be v1beta1 or v1beta2"
    exit 1
    ;;
esac

if [[ -z "${HCLOUD_TOKEN:-}" ]]; then
    echo "Error: HCLOUD_TOKEN environment variable is not set or empty."
    exit 2
fi
SECRET=hcloud
NAME="$2"

BIN=./hack/tools/bin

secret_args=(
    create secret generic "$SECRET"
    --from-literal=hcloud="$HCLOUD_TOKEN"
)

if [[ -n $ROBOT ]]; then
    if [[ -z "${HETZNER_ROBOT_USER:-}" || -z "${HETZNER_ROBOT_PASSWORD:-}" ]]; then
        echo "Error: HETZNER_ROBOT_USER/HETZNER_ROBOT_PASSWORD must be set when using --robot."
        exit 2
    fi
    secret_args+=(--from-literal=robot-user="$HETZNER_ROBOT_USER")
    secret_args+=(--from-literal=robot-password="$HETZNER_ROBOT_PASSWORD")
fi

"$BIN"/kubectl "${secret_args[@]}" \
    --save-config --dry-run=client -o yaml | "$BIN"/kubectl apply -f -

if [[ -n $ROBOT ]]; then

    "$BIN"/kubectl create secret generic robot-ssh \
        --from-literal=sshkey-name="$SSH_KEY_NAME" \
        --from-file=ssh-privatekey="$HETZNER_SSH_PRIV_PATH" \
        --from-file=ssh-publickey="$HETZNER_SSH_PUB_PATH" \
        --save-config --dry-run=client -o yaml | "$BIN"/kubectl apply -f -

fi

TEMPLATE_DIR=templates/cluster-templates/"$API_VERSION"/"$NAME"
if [[ ! -e $TEMPLATE_DIR ]]; then
    echo "$TEMPLATE_DIR does not exist. Check NAME argument of this script."
    exit 1
fi

mkdir -p generated/"$API_VERSION"

if [[ -z $CLUSTER_NAME ]]; then
    echo "CLUSTER_NAME not set"
    exit 1
fi
UNIQ="$API_VERSION/$NAME--$CLUSTER_NAME"
"$BIN"/kustomize build templates/cluster-templates/"$API_VERSION/$NAME" \
    --load-restrictor LoadRestrictionsNone >>generated/"$UNIQ"--kustomized.yaml

"$BIN"/clusterctl generate yaml --from generated/"$UNIQ"--kustomized.yaml \
    >generated/"$UNIQ"--rendered.yaml
echo "Created generated/$UNIQ--rendered.yaml"

echo "Applying manifests"
"$BIN"/kubectl apply --validate=true -f generated/"$UNIQ"--rendered.yaml

make wait-and-get-secret
make install-cilium-in-wl-cluster
make install-ccm-in-wl-cluster
