#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

echo '--> Starting cilium requirements.'
# mount bpfs for cilium
cat > /etc/systemd/system/sys-fs-bpf.mount <<EOF
[Unit]
Description=Cilium BPF mounts
Documentation=https://docs.cilium.io/
DefaultDependencies=no
Before=local-fs.target umount.target
After=swap.target

[Mount]
What=bpffs
Where=/sys/fs/bpf
Type=bpf
Options=rw,nosuid,nodev,noexec,relatime,mode=700

[Install]
WantedBy=multi-user.target
EOF

systemctl enable sys-fs-bpf.mount

# Cilium 1.9 Requirements
# Set up required sysctl params, these persist across reboots.
cat > /etc/sysctl.d/99-cilium.conf <<EOF
net.ipv4.conf.lxc*.rp_filter = 0
EOF



