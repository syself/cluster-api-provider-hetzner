#!/usr/bin/env bash
# Bash Strict Mode: https://github.com/guettli/bash-strict-mode
trap 'echo -e "\n🤷 🚨 🔥 Warning: A command has failed. Exiting the script. Line was ($0:$LINENO): $(sed -n "${LINENO}p" "$0" 2>/dev/null || true) 🔥 🚨 🤷 "; exit 3' ERR
set -Eeuo pipefail

links=$(find docs/ -name '*.md' -print0 | xargs -r0 grep -Pi '\(/(?!(docs|https?:))')
if [[ ! -z $links ]]; then
    echo "Links like this are not supported by the frontend which renders the docs:"
    echo "   [foo](/starts-with-slash)"
    echo "Change them to:"
    echo "   [foo](https://github.com/syself/cluster-api-provider-hetzner/blob/main/starts-with-slash)"
    echo "broken lines:"
    echo "$links"
    echo
    echo "linting failed"
    exit 1
fi

# Show current version
lychee --version

if [[ -z $GITHUB_TOKEN ]]; then
    echo "GITHUB_TOKEN is not set"
    exit 1
fi
lychee --verbose --config .lychee.toml --cache \
    ./*.md ./docs/**/*.md 2>&1 | grep -vP '\[(200|EXCLUDED)\]'
