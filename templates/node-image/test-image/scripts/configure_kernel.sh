#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

rpm --import https://www.elrepo.org/RPM-GPG-KEY-elrepo.org

dnf -y install https://www.elrepo.org/elrepo-release-8.el8.elrepo.noarch.rpm

# Install Kernel >5.10 needed for cilium TPROXY # Link to get newer kernel, use the ml-core Version: https://elrepo.org/linux/kernel/el8/x86_64/RPMS/
dnf --enablerepo=elrepo-kernel install -y kernel-ml

# update grub to use latest kernel
grub2-set-default 0
grub2-mkconfig -o /boot/grub2/grub.cfg


