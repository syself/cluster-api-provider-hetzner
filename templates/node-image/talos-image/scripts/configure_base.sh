#!/bin/sh

apt-get install -y wget
wget -O /tmp/talos.raw.xz ${IMAGE_URL}
xz -d -c /tmp/talos.raw.xz | dd of=/dev/sda && sync