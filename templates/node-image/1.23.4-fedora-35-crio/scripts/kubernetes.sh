#!/bin/sh

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

cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF
# Check actual version: https://github.com/kubernetes/kubernetes/releases must be same as cri-o
KUBERNETES_VERSION=1.23.4 # https://kubernetes.io/releases/#release-history

dnf install --setopt=obsoletes=0 -y kubelet-0:$KUBERNETES_VERSION-0 kubeadm-0:$KUBERNETES_VERSION-0 kubectl-0:$KUBERNETES_VERSION-0 python3-dnf-plugin-versionlock bash-completion --disableexcludes=kubernetes
dnf versionlock kubelet kubectl kubeadm
systemctl enable kubelet


# Set up required sysctl params, these persist across reboots.
cat > /etc/sysctl.d/99-kubernetes-cri.conf <<EOF
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

cat > /etc/sysctl.d/99-kubelet.conf <<EOF
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
EOF


systemctl start crio
kubeadm config images pull --kubernetes-version $KUBERNETES_VERSION

dnf install -y policycoreutils-python-utils

semanage fcontext -a -t container_file_t /var/lib/etcd
mkdir -p /var/lib/etcd
restorecon -rv /var /etc

# enable completion
echo 'source <(kubectl completion bash)' >>~/.bashrc

# set the kubeadm default path for kubeconfig
echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >>~/.bashrc
