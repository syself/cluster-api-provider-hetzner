#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

echo '--> Starting CRI Script.'
# Prerequisites
cat <<EOF | tee /etc/modules-load.d/crio.conf
overlay
br_netfilter
EOF

modprobe overlay
modprobe br_netfilter

sysctl --system

RUNC=v1.0.3       # https://github.com/opencontainers/runc/releases
CRUN=1.3          # https://github.com/containers/crun/releases
CONMON=v2.0.31    # https://github.com/containers/conmon/releases
CRIO=1.22         # https://github.com/cri-o/cri-o/releases
## Remember to check CRI-O Configuration updates when updating CRI-O
CRI_TOOLS=v1.22.0 # https://github.com/kubernetes-sigs/cri-tools/releases

# Install runc
wget https://github.com/opencontainers/runc/releases/download/$RUNC/runc.amd64 -O /usr/local/sbin/runc && chmod +x /usr/local/sbin/runc

# Install crun
wget https://github.com/containers/crun/releases/download/$CRUN/crun-$CRUN-linux-amd64 -O /usr/local/sbin/crun && chmod +x /usr/local/sbin/crun

# Install conmon
wget https://github.com/containers/conmon/releases/download/$CONMON/conmon-x86.zip -O conmon.zip && unzip conmon.zip -d conmon && mv conmon/bin/conmon /usr/local/bin/conmon && chmod +x /usr/local/bin/conmon
rm -rf conmon.zip conmon

# install cri-o https://github.com/cri-o/cri-o/blob/main/install.md#fedora-31-or-later
# curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable.repo https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/CentOS_8/devel:kubic:libcontainers:stable.repo
# curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable:cri-o:$CRIO.repo https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable:cri-o:$CRIO/CentOS_8/devel:kubic:libcontainers:stable:cri-o:$CRIO.repo
# dnf -y install cri-o
dnf -y module enable cri-o:$CRIO
dnf -y install cri-o

# cri-tool https://github.com/kubernetes-sigs/cri-tools
# Install crictl
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$CRI_TOOLS/crictl-$CRI_TOOLS-linux-amd64.tar.gz
tar zxvf crictl-$CRI_TOOLS-linux-amd64.tar.gz -C /usr/local/bin 
rm -f crictl-$CRI_TOOLS-linux-amd64.tar.gz

# remove default CNIs
rm -f /etc/cni/net.d/100-crio-bridge.conf /etc/cni/net.d/200-loopback.conf


# CRI-O Configuration
# https://github.com/cri-o/cri-o/blob/master/docs/crio.conf.5.md

mkdir -p /etc/crio/crio.conf.d && cat > /etc/crio/crio.conf.d/02-cgroup-manager.conf  <<"EOF"
[crio.runtime]
default_runtime = "crun"
conmon = "/usr/local/bin/conmon"
conmon_env = [
    "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
]
selinux = false
seccomp_profile = ""
apparmor_profile = "crio-default"
default_capabilities = [
    "CHOWN",
    "DAC_OVERRIDE",
    "FSETID",
    "FOWNER",
    "SETGID",
    "SETUID",
    "SETPCAP",
    "NET_BIND_SERVICE",
    "KILL",
    "MKNOD",
]

[crio.runtime.runtimes.runc]
runtime_path = ""
runtime_type = "oci"
runtime_root = "/run/runc"

[crio.runtime.runtimes.crun]
runtime_path = "/usr/local/sbin/crun"
runtime_type = "oci"
runtime_root = "/run/crun"

EOF

#Registries
# https://github.com/containers/image/blob/master/docs/containers-registries.conf.5.md


# Policy for CRI-O
# https://github.com/containers/image/blob/master/docs/containers-policy.json.5.md



# Storage Configuartion for CRI-O
# https://github.com/containers/storage/blob/master/docs/containers-storage.conf.5.md


# enable systemd service after next boot
systemctl enable crio.service
systemctl daemon-reload
systemctl enable crio


# Check if cgroup v2 is enabled: stat -c %T -f /sys/fs/cgroup output should be: cgroup2fs and test: cat /sys/fs/cgroup/cgroup.controllers output: cpuset cpu io memory hugetlb pids rdma misc
 