#!/bin/bash
# Copyright 2023 The Kubernetes Authors.
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

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 VAR1 VAR2 ..."
  exit 1
fi

missing_vars=()
for varname in "$@"; do
  eval varvalue="\$$varname"
  if [ -z "$varvalue" ]; then
    missing_vars+=("$varname")
  fi
done

if [ ${#missing_vars[@]} -gt 0 ]; then
  echo "Missing or empty environment variables: ${missing_vars[*]}"
  exit 1
fi

if [[ -n "${HETZNER_SSH_PUB:-}" ]]; then
  if ! decoded_pub="$(printf '%s' "$HETZNER_SSH_PUB" | base64 --decode 2>/dev/null)"; then
    echo "env var HETZNER_SSH_PUB seems wrong. It should be the base64-encoded pub key. Guess: update .envrc"
    exit 1
  fi
  if ! printf '%s' "$decoded_pub" | grep -q -- 'ssh-'; then
    echo "env var HETZNER_SSH_PUB seems wrong. It should be the base64-encoded pub key. Guess: update .envrc"
    exit 1
  fi
fi

if [[ -n "${HETZNER_SSH_PRIV:-}" ]]; then
  if ! decoded_priv="$(printf '%s' "$HETZNER_SSH_PRIV" | base64 --decode 2>/dev/null)"; then
    echo "env var HETZNER_SSH_PRIV seems wrong. It should be the base64-encoded priv key. Guess: update .envrc"
    exit 1
  fi
  if ! printf '%s' "$decoded_priv" | grep -q -- 'OPENSSH'; then
    echo "env var HETZNER_SSH_PRIV seems wrong. It should be the base64-encoded priv key. Guess: update .envrc"
    exit 1
  fi
fi

# Ensure env var KUBERNETES_VERSION is not outdated.
if [[ -n "${KUBERNETES_VERSION:-}" ]]; then
  min_kubernetes_version="1.33.6"
  kubernetes_version="${KUBERNETES_VERSION#v}"

  if [[ ! "$kubernetes_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "env var KUBERNETES_VERSION should look like v<major>.<minor>.<patch> or <major>.<minor>.<patch>, got '$KUBERNETES_VERSION'."
    exit 1
  fi

  if [[ "$(printf '%s\n%s\n' "$min_kubernetes_version" "$kubernetes_version" | sort -V | head -n 1)" != "$min_kubernetes_version" ]]; then
    echo "KUBERNETES_VERSION must be >= v${min_kubernetes_version}, got '$KUBERNETES_VERSION'."
    exit 1
  fi
fi

# Ensure that no outdated hcloud machine types get used.
for varname in "$@"; do
  if [ "$varname" = "HCLOUD_CONTROL_PLANE_MACHINE_TYPE" ] || [ "$varname" = "HCLOUD_WORKER_MACHINE_TYPE" ]; then
    deprecated_types=(cx22 cx32 cx42 cx52 cpx11 cpx21 cpx31 cpx41 cpx51)
    for deprecated in "${deprecated_types[@]}"; do
      if [[ "${!varname}" == *"$deprecated"* ]]; then
        echo "$varname contains deprecated type '$deprecated'."
        echo "Deprecated types: ${deprecated_types[*]}"
        exit 1
      fi
    done
  fi
done
