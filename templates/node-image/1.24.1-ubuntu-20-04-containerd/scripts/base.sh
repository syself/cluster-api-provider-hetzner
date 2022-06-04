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

echo '--> Starting Base Installation.'
# Set locale
localectl set-locale LANG=en_US.UTF-8 
localectl set-locale LANGUAGE=en_US.UTF-8

# update all packages
apt-get update -y

# install basic tooling
apt-get -y install \
    at jq unzip wget socat mtr logrotate apt-transport-https

# Install yq
YQ_VERSION=v4.20.1 #https://github.com/mikefarah/yq
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

