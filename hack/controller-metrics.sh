#!/usr/bin/env bash

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

set -Eeuo pipefail
trap 'echo "⚠️ An error occurred; aborting controller metrics collection" >&2; exit 1' ERR

hack_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
controller_deployment="caph-controller-manager"
metrics_local_port=18080
pf_pid=

current_cluster="$(kubectl config current-context 2>/dev/null || true)"
current_cluster="${current_cluster:-unknown}"

controller_kinds=(
    HCloudMachine
    HCloudMachineTemplate
    HCloudRemediation
    HCloudRemediationTemplate
    HetznerBareMetalHost
    HetznerBareMetalMachine
    HetznerBareMetalMachineTemplate
    HetznerBareMetalRemediation
    HetznerBareMetalRemediationTemplate
    HetznerCluster
    HetznerClusterTemplate
)

cache_file="${XDG_CACHE_HOME:-$HOME/.cache}/caph-controller-metrics.cache"
declare -A cache_total
declare -A cache_ts
declare -A persisted_data

function cleanup_port_forward() {
    if [[ -n "${pf_pid:-}" ]]; then
        kill "$pf_pid" >/dev/null 2>&1 || true
        wait "$pf_pid" >/dev/null 2>&1 || true
        pf_pid=
    fi
}

trap cleanup_port_forward EXIT

function namespace_of_controller() {
    "$hack_dir"/get-namespace-of-deployment.sh "$controller_deployment"
}

function pod_of_controller() {
    local ns="$1"
    "$hack_dir"/get-leading-pod.sh "$controller_deployment" "$ns"
}

function port_forward_metrics_endpoint() {
    local ns="$1"
    local pod="$2"
    kubectl -n "$ns" port-forward --address 127.0.0.1 "$pod" "$metrics_local_port":8080 >/dev/null &
    pf_pid=$!
    # Give port-forward a moment to establish.
    sleep 0.5
}

function scrape_metrics() {
    curl -fsSL "http://127.0.0.1:${metrics_local_port}/metrics"
}

function parse_metric() {
    local metric="$1"
    local mode="$2"
    local ctl="$3"

    awk -v ctrl="${ctl}" -v met="$metric" -v mode="$mode" '
        BEGIN { sum = 0 }
        {
            if ($1 ~ /^#/) {
                next
            }
            name = $1
            sub(/\{.*$/, "", name)
            if (name == met && index($0, "controller=\"" ctrl "\"") > 0) {
                if (mode == "single") {
                    print $NF
                    exit
                }
                sum += $NF
            }
        }
        END {
            if (mode != "single") {
                print sum
            }
        }
    ' <<< "$metrics"
}

function load_cache() {
    if [[ -r "$cache_file" ]]; then
        while read -r cluster controller total ts; do
            if [[ -z "$ts" ]]; then
                # old-format entry (cluster field missing) or corrupted line
                continue
            fi
            key="${cluster}/${controller}"
            persisted_data[$key]="$total,$ts"
            if [[ "$cluster" == "$current_cluster" ]]; then
                cache_total[$controller]=$total
                cache_ts[$controller]=$ts
            fi
        done < "$cache_file"
    fi
}

function persist_cache() {
    mkdir -p "$(dirname "$cache_file")"
    {
        for kind in "${controller_kinds[@]}"; do
            ctrl="$(printf '%s' "$kind" | awk '{print tolower($0)}')"
            cache_total[$ctrl]=${cache_total[$ctrl]:-0}
            cache_ts[$ctrl]=${cache_ts[$ctrl]:-$run_timestamp}
            persisted_data["$current_cluster/$ctrl"]="${cache_total[$ctrl]},${cache_ts[$ctrl]}"
        done

        for key in $(printf '%s\n' "${!persisted_data[@]}" | sort); do
            cluster="${key%%/*}"
            ctrl="${key#*/}"
            IFS=',' read -r total ts <<< "${persisted_data[$key]}"
            printf '%s %s %s %s\n' "$cluster" "$ctrl" "$total" "$ts"
        done
    } > "$cache_file"
}

function print_controller_metrics() {
    local kind="$1"
    local ctrl_name
    local total duration_sum duration_count avg
    local rate_msg prev_total prev_ts delta delta_time

    ctrl_name="$(printf '%s' "$kind" | awk '{print tolower($0)}')"
    total="$(parse_metric "controller_runtime_reconcile_total" "sum" "$ctrl_name")"
    duration_sum="$(parse_metric "controller_runtime_reconcile_time_seconds_sum" "single" "$ctrl_name")"
    duration_count="$(parse_metric "controller_runtime_reconcile_time_seconds_count" "single" "$ctrl_name")"

    total=${total:-0}

    printf '\nController "%s" (kind=%s)\n' "$ctrl_name" "$kind"
    printf 'Reconcile invocations: %s\n' "$total"

    if [[ -n "$duration_count" ]] && awk -v count="$duration_count" 'BEGIN { exit count>0 ? 0 : 1 }'; then
        avg=$(awk -v sum="${duration_sum:-0}" -v count="$duration_count" 'BEGIN { printf "%.3f", sum/count }')
        printf 'Average reconcile duration: %ss (count=%s, total=%ss)\n' "$avg" "$duration_count" "${duration_sum:-0}"
    else
        echo "Average reconcile duration: n/a (no recorded runs yet)"
    fi

    prev_total=${cache_total[$ctrl_name]:-}
    prev_ts=${cache_ts[$ctrl_name]:-}
    if [[ -n "$prev_total" && -n "$prev_ts" ]]; then
        delta_time=$((run_timestamp - prev_ts))
        if (( delta_time > 0 )); then
            delta=$((total - prev_total))
            if (( delta >= 0 )); then
                rate_msg=$(awk -v dt="$delta" -v dx="$delta_time" 'BEGIN { printf "%.3f", (dx == 0 ? 0 : dt/dx) }')
                printf 'Rate: %s calls/s (delta=%s over %ds)\n' "$rate_msg" "$delta" "$delta_time"
            else
                echo "Rate: counter reset detected"
            fi
        else
            echo "Rate: n/a (time did not advance)"
        fi
    else
        echo "Rate: n/a (no previous sample)"
    fi

    cache_total[$ctrl_name]=$total
    cache_ts[$ctrl_name]=$run_timestamp
}

load_cache
ns="$(namespace_of_controller)"
if [[ -z "$ns" ]]; then
    echo "Unable to determine namespace of $controller_deployment" >&2
    exit 1
fi

pod="$(pod_of_controller "$ns")"
if [[ -z "$pod" ]]; then
    echo "Unable to determine pod of $controller_deployment in namespace $ns" >&2
    exit 1
fi

port_forward_metrics_endpoint "$ns" "$pod"
metrics="$(scrape_metrics)"
run_timestamp=$(date +%s)
run_timestamp=$(date +%s)

for kind in "${controller_kinds[@]}"; do
    print_controller_metrics "$kind"
done

persist_cache
