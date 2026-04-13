#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
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

# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

if [[ ${RUNNING_IN_CONTAINER:-} != "true" ]]; then
    echo "This script should be called via the builder image"
    exit 1
fi

links=""
grep_status=0

links=$(grep -rPi --include='*.md' '\(/(?!docs)' docs/) || grep_status=$?
case $grep_status in
0) : ;;                   # Unsupported links were found. Handle that below via $links.
1) links="" ;;            # Nothing found. That is valid for this lint check.
*) exit "$grep_status" ;; # grep hit a real error. Preserve the original exit status.
esac

if [[ -n $links ]]; then
    echo "Links like this are not supported by the frontend which renders the docs:"
    echo "   [foo](/some-dir-in-git-root/foo)"
    echo "Change them to:"
    echo "   [foo](https://github.com/syself/cluster-api-provider-hetzner/blob/main/starts-with-slash)"
    echo "   Links like this are ok: [some md file](/docs/.../foo.md)"
    echo
    echo "We can create links to files the 'docs' directory. Preview URLs in CI work."
    echo "But currently, we can't create preview links to files outside the 'docs' dir."
    echo
    echo "broken lines:"
    echo "$links"
    echo
    echo "linting failed"
    exit 1
fi

# Show current version
lychee --version

if [[ -z ${GITHUB_TOKEN:-} ]]; then
    echo "GITHUB_TOKEN is not set"
    exit 1
fi
lychee --verbose --config .lychee.toml --root-dir . --cache \
    ./*.md ./docs/**/*.md 2>&1 | sed -nE '/\[(200|EXCLUDED)\]/!p'
