#!/bin/sh
PACKER=$1
shift
export HCLOUD_TOKEN=test
exec $PACKER validate "$@"
