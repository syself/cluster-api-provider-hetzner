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
trap 'echo -e "\nWarning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

if ! command -v kustomize | grep hack/tools/bin; then
    echo "fix $PATH: kustomize should be from hack/tools/bin"
    exit 1
fi

rm -f "$HETZNER_TEMPLATES"/v1beta1/cluster-template*.yaml

if [ -z "${HETZNER_ROBOT_USER:-}" ]; then

    if [ -n "${HETZNER_SSH_PUB:-}" ] ||
        [ -n "${HETZNER_SSH_PRIV:-}" ] ||
        [ -n "${HETZNER_ROBOT_PASSWORD:-}" ]; then
        echo "Environment variables HETZNER_SSH_PUB HETZNER_SSH_PRIV HETZNER_ROBOT_PASSWORD should not be set."
        exit 1
    fi
    echo "No HETZNER_ROBOT_USER set, setting values to dummy values"
    HETZNER_ROBOT_USER="dummy-HETZNER_ROBOT_USER"
    HETZNER_ROBOT_PASSWORD="dummy-HETZNER_ROBOT_PASSWORD"
    HETZNER_SSH_PUB=$(echo -n "dummy-HETZNER_SSH_PUB" | base64 -w0)
    HETZNER_SSH_PRIV=$(echo -n "dummy-HETZNER_SSH_PRIV" | base64 -w0)
fi

echo -n "$HETZNER_SSH_PUB" | base64 -d >tmp_ssh_pub_enc
echo -n "$HETZNER_SSH_PRIV" | base64 -d >tmp_ssh_priv_enc
kubectl create secret generic robot-ssh --from-literal=sshkey-name=shared-2024-07-08 --from-file=ssh-privatekey=tmp_ssh_priv_enc --from-file=ssh-publickey=tmp_ssh_pub_enc --dry-run=client -o yaml >data/infrastructure-hetzner/v1beta1/cluster-template-hetzner-secret.yaml
rm tmp_ssh_pub_enc tmp_ssh_priv_enc

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-k8s-upgrade --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-k8s-upgrade.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-k8s-upgrade-kcp-scale-in --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-k8s-upgrade-kcp-scale-in.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-hcloud-feature-loadbalancer-off --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-hcloud-feature-loadbalancer-off.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-hcloud-feature-load-balancer-extra-services --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-hcloud-feature-load-balancer-extra-services.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-hcloud-feature-placement-groups --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-hcloud-feature-placement-groups.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-network --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-network.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-kcp-remediation --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-kcp-remediation.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-md-remediation --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-md-remediation.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-node-drain --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-node-drain.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-hetzner-baremetal --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-hetzner-baremetal.yaml

kustomize build "$HETZNER_TEMPLATES"/v1beta1/cluster-template-hetzner-baremetal-feature-raid-setup --load-restrictor LoadRestrictionsNone >"$HETZNER_TEMPLATES"/v1beta1/cluster-template-hetzner-baremetal-feature-raid-setup.yaml

sed -i "s/robot-user_secret_placeholder/$(echo -n "$HETZNER_ROBOT_USER" | base64 -w0)/" "$HETZNER_TEMPLATES"/v1beta1/*.yaml
sed -i "s/robot-password_secret_placeholder/$(echo -n "$HETZNER_ROBOT_PASSWORD" | base64 -w0)/" "$HETZNER_TEMPLATES"/v1beta1/*.yaml

sed -i "s/hcloud_secret_placeholder/$(echo -n "$HCLOUD_TOKEN" | base64 -w0)/" "$HETZNER_TEMPLATES"/v1beta1/*.yaml

echo "Generated yaml files in $HETZNER_TEMPLATES/v1beta1/"
echo
