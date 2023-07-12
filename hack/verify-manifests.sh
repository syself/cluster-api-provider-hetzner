#!/usr/bin/env bash

# Copyright 2014 The Kubernetes Authors.
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

if [[ "$(git diff | wc -l)" -gt 0 ]]; then echo ">>> make generate-manifests generated untracked files which have not been committed" && exit 1; fi

test -z "$(git ls-files --others --exclude-standard 2> /dev/null)" || (echo ">>> make generate-manifests generated untracked files" && exit 1)
