#!/usr/bin/env bash
# get_root_path returns the root path of the project source tree
get_root_path() {
    git rev-parse --show-toplevel
}

# cd_root_path cds to the root path of the project source tree
cd_root_path() {
    cd "$(get_root_path)" || exit
}
