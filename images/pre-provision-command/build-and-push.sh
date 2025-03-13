#!/usr/bin/env bash
# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo "Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0")"; exit 3' ERR
set -Eeuo pipefail

DIR="$(dirname "$0")"

docker build -t ghcr.io/syself/caph-staging:pre-provision-command "$DIR"

docker push ghcr.io/syself/caph-staging:pre-provision-command
