#!/bin/bash

# Copyright 2024 The Kubernetes Authors.
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

shopt -s nullglob # Ensure the glob pattern expands to nothing if no files match

files=(/sys/class/block/md*/md/degraded) # Store the matched files in an array

if [ ${#files[@]} -eq 0 ]; then
    echo "No mdraid found"
    exit 1
fi

fail=0

for file in /sys/class/block/md*/md/degraded; do
    state=$(cat $file)
    if [[ "$state" != "0" ]]; then
        fail=1
        echo "mdraid is degraded! $file: $state"
        continue
    fi
done
if [ $fail -ne 0 ]; then
    exit 1
fi
echo "No degraded mdraid found."
