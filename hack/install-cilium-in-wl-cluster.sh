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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

# Deploy cilium
helm repo add cilium https://helm.cilium.io/
helm repo update cilium

if [[ -z ${WORKER_CLUSTER_KUBECONFIG:-} ]]; then
	echo "env var WORKER_CLUSTER_KUBECONFIG is not set"
	exit 1
fi

if [[ ! -e $WORKER_CLUSTER_KUBECONFIG ]]; then
	echo "WORKER_CLUSTER_KUBECONFIG=$WORKER_CLUSTER_KUBECONFIG is empty or does not exist"
	exit 1
fi

KUBECONFIG=$WORKER_CLUSTER_KUBECONFIG helm upgrade \
	--install cilium cilium/cilium --version 1.18.1 \
	--namespace kube-system \
	-f templates/cilium/cilium.yaml
