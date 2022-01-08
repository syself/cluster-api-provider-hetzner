#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

echo '--> Starting Base Installation.'
# Set locale
localectl set-locale LANG=en_US.UTF-8 
localectl set-locale LANGUAGE=en_US.UTF-8

# update all packages
dnf update -y

# install basic tooling
dnf -y install \
    at jq unzip wget socat mtr logrotate firewalld

# Install yq
YQ_VERSION=v4.16.1 #https://github.com/mikefarah/yq
YQ_BINARY=yq_linux_amd64
wget https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${YQ_BINARY} -O /usr/bin/yq &&\
    chmod +x /usr/bin/yq

echo '--> Starting Base Configuration.'

## disable swap
sed -i '/swap/d' /etc/fstab



echo '--> Starting Logrotate.' 
# Content from: https://github.com/kubernetes/kubernetes/blob/master/cluster/gce/gci/configure-helper.sh#L509

cat > /etc/logrotate.d/allvarlogs <<"EOF"
/var/log/*.log {
    rotate 5
    copytruncate
    missingok
    notifempty
    compress
    maxsize 25M
    daily
    dateext
    dateformat -%Y%m%d-%s
    create 0644 root root
}
EOF

cat > /etc/logrotate.d/allpodlogs <<"EOF"
/var/log/pods/*/*.log {
    rotate 3
    copytruncate
    missingok
    notifempty
    compress
    maxsize 5M
    daily
    dateext
    dateformat -%Y%m%d-%s
    create 0644 root root
}

EOF

