#!/usr/bin/env bash

set -euo pipefail

echo "WARNING! Are you sure to erase all container, images & volumes? [y/N]"
read -r -p "" response
case "$response" in
        [yY][eE][sS]|[yY]) 
            true
            ;;
        *)
            false
            return 2
            ;;
    esac

echo "Delete is in progress..." 
docker kill $(docker ps -q) || true
docker rm $(docker ps -a -q) || true
docker volume rm $(docker volume ls -q) || true
docker image prune -af || true
echo "Done"
