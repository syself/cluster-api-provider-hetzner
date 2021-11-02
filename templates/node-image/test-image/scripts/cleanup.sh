#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

# Ensure we don't leave SSH host keys
rm -rf /etc/ssh/ssh_host_*


# Remove build tools
dnf module -y remove go-toolset

dnf remove -y \
  device-mapper-devel \
  make \
  glib2-devel \
  glibc-devel \
  glibc-static \
  go \
  gpgme-devel \
  libassuan-devel \
  libgpg-error-devel \
  libseccomp-devel \
  libselinux-devel \
  pkgconfig \
  pkgconf-pkg-config

