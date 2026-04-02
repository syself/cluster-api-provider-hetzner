#!/bin/sh

# Copyright 2023 The Kubernetes Authors.
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

set -eu

SRC_PATH=/src/cluster-api-provider-hetzner
makefile="$SRC_PATH/Makefile"
if [ ! -e $makefile ]; then
    echo "Error: $makefile does not exist."
    echo "Be sure to use:"
    # shellcheck disable=SC2016
    echo '   docker run  --rm -v $HOME/go/pkg:/go/pkg -v $(pwd):/src/cluster-api-provider-hetzner  ghcr.io/syself/caph-builder:XXX makefile-target'
    exit 1
fi

# We autodetect the UID/GID of the host, and create a Linux user inside the container. This way the
# cache between the host and inside the container can be shared. This avoids permission problems.
uid=$(stat --format="%u" "${SRC_PATH}")
gid=$(stat --format="%g" "${SRC_PATH}")
echo "caph:x:${uid}:${gid}::${SRC_PATH}:/bin/bash" >>/etc/passwd
echo "caph:*:::::::" >>/etc/shadow
echo "caph	ALL=(ALL)	NOPASSWD: ALL" >>/etc/sudoers

# This chown is needed. Otherwise /home/runner/go/pkg will suddenly belong to the root user. We want
# to avoid this permission change in the file system of the host (outside the container).
mkdir -p /go/pkg
# Use the host-mounted GOPATH cache to avoid filling the container filesystem.
export GOPATH=/go
export GOCACHE=/go/pkg/go-build
export GOMODCACHE=/go/pkg/mod
mkdir -p "${GOCACHE}" "${GOMODCACHE}"
# Do not add "-R". This would add a overhead of 15 seconds for each start of the container.
chown "$uid":"$gid" /go /go/pkg "${GOCACHE}" "${GOMODCACHE}"

su caph -c "PATH=${PATH} GOPATH=/go GOCACHE=/go/pkg/go-build GOMODCACHE=/go/pkg/mod make -C ${SRC_PATH} BUILD_IN_CONTAINER=false $*"
