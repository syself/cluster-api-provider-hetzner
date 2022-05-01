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

echo '--> Starting CRI Script.'
# Prerequisites
cat <<'EOF' | tee /etc/modules-load.d/containerd.conf
overlay
br_netfilter
EOF

modprobe overlay
modprobe br_netfilter

# Setting up sysctl properties
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
echo fs.inotify.max_user_instances=8192 | sudo tee -a /etc/sysctl.conf
echo vm.max_map_count=524288 | sudo tee -a /etc/sysctl.conf

# Set up required sysctl params, these persist across reboots.
cat > /etc/sysctl.d/99-kubernetes-cri.conf <<'EOF'
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# Required by protectedKernelDefaults=true
cat > /etc/sysctl.d/99-kubelet.conf <<'EOF'
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
EOF

# Apply sysctl params without reboot
sysctl --system

CRUN=1.4.5        # https://github.com/containers/crun/releases
CONTAINERD=1.6.2  # https://github.com/containerd/containerd/releases

# Install containerd
wget https://github.com/containerd/containerd/releases/download/v${CONTAINERD}/cri-containerd-cni-${CONTAINERD}-linux-amd64.tar.gz
wget https://github.com/containerd/containerd/releases/download/v${CONTAINERD}/cri-containerd-cni-${CONTAINERD}-linux-amd64.tar.gz.sha256sum
sha256sum --check cri-containerd-cni-${CONTAINERD}-linux-amd64.tar.gz.sha256sum
tar --no-overwrite-dir -C / -xzf cri-containerd-cni-${CONTAINERD}-linux-amd64.tar.gz

# Cleanup 
rm -f cri-containerd-cni-${CONTAINERD}-linux-amd64.tar.gz cri-containerd-cni-${CONTAINERD}-linux-amd64.tar.gz.sha256sum

# Install crun
wget https://github.com/containers/crun/releases/download/$CRUN/crun-$CRUN-linux-amd64 -O /usr/local/sbin/crun && chmod +x /usr/local/sbin/crun

mkdir -p /etc/containerd

cat <<'EOF' | sudo tee /etc/containerd/config.toml
version = 2
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
  runtime_type = "io.containerd.runc.v2"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
  SystemdCgroup = true
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.crun]
  runtime_type = "io.containerd.runc.v2"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.crun.options]
  BinaryName = "crun"
  Root = "/usr/local/sbin"
  SystemdCgroup = true
[plugins."io.containerd.grpc.v1.cri".containerd]
  default_runtime_name = "crun"
[plugins."io.containerd.runtime.v1.linux"]
  runtime = "crun"
  runtime_root = "/usr/local/sbin"
EOF

rm -f /etc/cni/net.d/10-containerd-net.conflist

# Sets permission accordingly to CIS Benchmark
chmod -R 644 /etc/cni
chown -R root:root /etc/cni 

# enable systemd service after next boot
systemctl daemon-reload
systemctl enable containerd
systemctl start containerd