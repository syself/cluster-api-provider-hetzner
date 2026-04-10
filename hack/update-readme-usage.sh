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

readonly REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly README_FILE="$REPO_ROOT/README.md"
readonly START_MARKER="<!-- BEGIN MAIN BINARY USAGE -->"
readonly END_MARKER="<!-- END MAIN BINARY USAGE -->"

cd "$REPO_ROOT"

if ! grep -Fq "$START_MARKER" "$README_FILE"; then
	echo "Missing README marker: $START_MARKER"
	exit 1
fi

if ! grep -Fq "$END_MARKER" "$README_FILE"; then
	echo "Missing README marker: $END_MARKER"
	exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT

binary_path="$tmp_dir/cluster-api-provider-hetzner"
usage_output_file="$tmp_dir/usage.txt"
replacement_file="$tmp_dir/replacement.txt"
updated_readme_file="$tmp_dir/README.md"

go build -o "$binary_path" .
"$binary_path" >"$usage_output_file"

{
	echo "$START_MARKER"
	echo '```console'
	echo '$ cluster-api-provider-hetzner'
	cat "$usage_output_file"
	echo '```'
	echo "$END_MARKER"
} >"$replacement_file"

awk \
	-v start="$START_MARKER" \
	-v end="$END_MARKER" \
	-v replacement="$replacement_file" \
	'
BEGIN {
	while ((getline line < replacement) > 0) {
		replacement_text = replacement_text line ORS
	}
}
$0 == start {
	printf "%s", replacement_text
	in_block = 1
	found_start = 1
	next
}
$0 == end {
	in_block = 0
	found_end = 1
	next
}
!in_block {
	print
}
END {
	if (!found_start || !found_end) {
		exit 1
	}
}
' "$README_FILE" >"$updated_readme_file"

mv "$updated_readme_file" "$README_FILE"
