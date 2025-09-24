#!/usr/bin/env python3

# Copyright 2025 The Kubernetes Authors.
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

# Create markdown table from transitions:
# k logs deployments/caph-controller-manager | python3 hack/hcloud-image-url-command-states-markdown-from-logs.py
import sys, json
from collections import defaultdict

agg = defaultdict(lambda: [0, 0.0, float("inf"), float("-inf")])  # count,sum,min,max

for line in sys.stdin:
    try:
        o = json.loads(line)
        if not o.get("durationInState"):
            continue
        old, new, d = o.get("oldState"), o.get("newState"), o.get("durationInState")
        if old is None or new is None or d is None:
            continue
        d = float(d)
        k = (old, new)
        agg[k][0] += 1
        agg[k][1] += d
        agg[k][2] = min(agg[k][2], d)
        agg[k][3] = max(agg[k][3], d)
    except Exception:
        pass

custom_state_order = [
    "",
    "empty",
    "Initializing",
    "EnablingRescue",
    "BootingToRescue",
    "RunningImageCommand",
    "BootToRealOS",
]
order_index = {s: i for i, s in enumerate(custom_state_order)}


def _state_rank(s: str) -> int:
    # states not in list are placed after the predefined ones, keeping alphabetical order among themselves
    base = order_index.get(s, len(custom_state_order))
    if base == len(custom_state_order):
        # offset plus alphabetical tiebreaker via tuple sort later
        return base
    return base


def _sort_key(item):
    (old, new), _ = item
    return (_state_rank(old), _state_rank(new), old, new)


rows = sorted(agg.items(), key=_sort_key)

if not rows:
    print("No provisioning state transitions detected (no lines with durationInState).")
    sys.exit(0)

# Markdown table header
print("| oldState | newState | avg(s) | min(s) | max(s) |")
print("|----------|----------|-------:|-------:|-------:|")
for (old, new), (cnt, sumd, mn, mx) in rows:
    avg = sumd / cnt if cnt else 0.0
    print(f"| {old} | {new} | {avg:.2f} | {mn:.2f} | {mx:.2f} |")
