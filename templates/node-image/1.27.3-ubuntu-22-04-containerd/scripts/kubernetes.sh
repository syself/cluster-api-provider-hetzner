#!/bin/bash

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

KUBERNETES_VERSION=1.27.3 # https://kubernetes.io/releases/#release-history
TRIMMED_KUBERNETES_VERSION=$(echo ${KUBERNETES_VERSION} | sed 's/^v//' | awk -F . '{print $1 "." $2}')
curl -fsSL https://pkgs.k8s.io/core:/stable:/v$TRIMMED_KUBERNETES_VERSION/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v$TRIMMED_KUBERNETES_VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list
apt-get update

# Check actual version: https://github.com/kubernetes/kubernetes/releases

apt-get install -y kubelet=$KUBERNETES_VERSION-1.1 kubeadm=$KUBERNETES_VERSION-1.1 kubectl=$KUBERNETES_VERSION-1.1  bash-completion
apt-mark hold kubelet kubectl kubeadm

systemctl enable kubelet

kubeadm config images pull --kubernetes-version $KUBERNETES_VERSION

# enable completion
echo 'source <(kubectl completion bash)' >>~/.bashrc

# set the kubeadm default path for kubeconfig
echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >>~/.bashrc


