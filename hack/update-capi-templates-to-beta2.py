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

#!/usr/bin/env python3
usage = """
Update kubeletExtraArgs mappings to name/value lists in CAPI YAML templates.

Usage:
    python update-capi-templates-to-beta2.py path/to/file.yaml

The script overwrites the input file in place.

Install hint:

python3 -m venv .venv
source .venv/bin/activate
pip install ruamel.yaml

Mass-usage:

# first check if git is clean, so that a revert is easy, then:

find templates -name '*.yaml' | xargs -n1 ./hack/update-capi-templates-to-beta2.py
"""

import argparse
import re
import sys
from pathlib import Path

try:
    from ruamel.yaml import YAML
    from ruamel.yaml.comments import CommentedMap, CommentedSeq
except ImportError as exc:  # pragma: no cover - runtime dependency guard
    sys.stderr.write(f"ruamel.yaml is required for this script.\n{usage}")
    raise


def convert_mapping_to_seq(mapping: CommentedMap) -> CommentedSeq:
    """Convert a mapping of kubeletExtraArgs into a list of name/value maps."""
    seq = CommentedSeq()
    for key, value in mapping.items():
        item = CommentedMap()
        item["name"] = key
        item["value"] = value
        seq.append(item)
    return seq


def duration_to_seconds(duration: str) -> int | None:
    """Convert simple duration strings like 180s/15m/1h/1d to seconds."""
    match = re.fullmatch(r"(\d+)([smhdSMHD]?)", duration.strip())
    if not match:
        return None
    value = int(match.group(1))
    unit = match.group(2).lower()
    if unit == "m":
        value *= 60
    elif unit == "h":
        value *= 60 * 60
    elif unit == "d":
        value *= 60 * 60 * 24
    return value


def transform_machine_health_check(mhc: CommentedMap) -> bool:
    """Update MachineHealthCheck spec to v1beta2 structure."""
    changed = False
    spec = mhc.get("spec")
    if not isinstance(spec, CommentedMap):
        return False

    checks = spec.get("checks")
    if not isinstance(checks, CommentedMap):
        checks = CommentedMap()

    # nodeStartupTimeout -> checks.nodeStartupTimeoutSeconds
    if "nodeStartupTimeout" in spec:
        timeout_val = spec.pop("nodeStartupTimeout")
        if isinstance(timeout_val, str):
            seconds = duration_to_seconds(timeout_val)
            if seconds is not None:
                checks["nodeStartupTimeoutSeconds"] = seconds
                changed = True
        else:
            checks["nodeStartupTimeoutSeconds"] = timeout_val
            changed = True

    # unhealthyConditions -> checks.unhealthyNodeConditions
    if "unhealthyConditions" in spec:
        conditions = spec.pop("unhealthyConditions")
        if isinstance(conditions, list):
            new_conditions = CommentedSeq()
            for cond in conditions:
                if not isinstance(cond, CommentedMap):
                    continue
                new_cond = CommentedMap()
                if "type" in cond:
                    new_cond["type"] = cond["type"]
                if "status" in cond:
                    new_cond["status"] = cond["status"]
                timeout = cond.get("timeout")
                if isinstance(timeout, str):
                    seconds = duration_to_seconds(timeout)
                    if seconds is not None:
                        new_cond["timeoutSeconds"] = seconds
                elif timeout is not None:
                    new_cond["timeoutSeconds"] = timeout
                if new_cond:
                    new_conditions.append(new_cond)
            if new_conditions:
                checks["unhealthyNodeConditions"] = new_conditions
                changed = True

    if checks:
        spec["checks"] = checks

    # maxUnhealthy -> remediation.triggerIf.unhealthyLessThanOrEqualTo
    if "maxUnhealthy" in spec:
        max_unhealthy = spec.pop("maxUnhealthy")
        remediation = spec.get("remediation")
        if not isinstance(remediation, CommentedMap):
            remediation = CommentedMap()
        trigger_if = remediation.get("triggerIf")
        if not isinstance(trigger_if, CommentedMap):
            trigger_if = CommentedMap()
        if "unhealthyLessThanOrEqualTo" not in trigger_if:
            trigger_if["unhealthyLessThanOrEqualTo"] = max_unhealthy
            changed = True
        remediation["triggerIf"] = trigger_if
        spec["remediation"] = remediation

    # remediationTemplate -> remediation.templateRef
    if "remediationTemplate" in spec:
        templ = spec.pop("remediationTemplate")
        if isinstance(templ, CommentedMap):
            remediation = spec.get("remediation")
            if not isinstance(remediation, CommentedMap):
                remediation = CommentedMap()
            templ_ref = remediation.get("templateRef")
            if not isinstance(templ_ref, CommentedMap):
                templ_ref = CommentedMap()
            for key in ("kind", "name", "apiVersion"):
                if key in templ and key not in templ_ref:
                    templ_ref[key] = templ[key]
            remediation["templateRef"] = templ_ref
            spec["remediation"] = remediation
            changed = True

    return changed


def infer_api_group(ref: CommentedMap) -> str | None:
    """Infer the apiGroup for a ref from apiVersion or kind heuristics."""
    api_version = ref.get("apiVersion")
    if isinstance(api_version, str) and "/" in api_version:
        return api_version.split("/", 1)[0]

    kind = ref.get("kind")
    if not isinstance(kind, str):
        return None

    explicit_kind_map = {
        "KubeadmControlPlane": "controlplane.cluster.x-k8s.io",
        "KubeadmControlPlaneTemplate": "controlplane.cluster.x-k8s.io",
        "KubeadmConfig": "bootstrap.cluster.x-k8s.io",
        "KubeadmConfigTemplate": "bootstrap.cluster.x-k8s.io",
        "KubeadmConfigTemplateList": "bootstrap.cluster.x-k8s.io",
    }
    if kind in explicit_kind_map:
        return explicit_kind_map[kind]

    # Infrastructure resources in this repo all live under infrastructure.cluster.x-k8s.io.
    infra_hints = ("HCloud", "Hetzner", "BareMetal", "BM")
    if (
        any(hint in kind for hint in infra_hints)
        or kind.endswith("Cluster")
        or kind.endswith("MachineTemplate")
    ):
        return "infrastructure.cluster.x-k8s.io"

    return None


def transform(node) -> bool:
    """
    Recursively walk the YAML structure, convert kubeletExtraArgs mappings,
    drop apiVersion inside infrastructureRef/controlPlaneRef/configRef blocks,
    ensure apiGroup is set on those refs, update extraArgs to name/value lists,
    and reshape MachineHealthCheck specs to v1beta2 schema.

    Returns True if any change was made.
    """
    changed = False

    if isinstance(node, CommentedMap):
        if "kubeletExtraArgs" in node:
            args_val = node["kubeletExtraArgs"]
            if isinstance(args_val, CommentedMap):
                node["kubeletExtraArgs"] = convert_mapping_to_seq(args_val)
                changed = True
            elif isinstance(args_val, CommentedSeq):
                # Already in desired format; nothing to do.
                pass
            else:
                raise TypeError(f"Unexpected kubeletExtraArgs type: {type(args_val)!r}")

        if "extraArgs" in node and isinstance(node["extraArgs"], CommentedMap):
            node["extraArgs"] = convert_mapping_to_seq(node["extraArgs"])
            changed = True

        for ref_key in ("infrastructureRef", "controlPlaneRef", "configRef"):
            if ref_key in node:
                ref_val = node[ref_key]
                if isinstance(ref_val, CommentedMap):
                    if "apiGroup" not in ref_val:
                        inferred = infer_api_group(ref_val)
                        if inferred:
                            ref_val["apiGroup"] = inferred
                            changed = True
                    if "apiVersion" in ref_val:
                        del ref_val["apiVersion"]
                        changed = True

        if "controlPlaneEndpoint" in node:
            cpe = node["controlPlaneEndpoint"]
            if (
                isinstance(cpe, CommentedMap)
                and str(cpe.get("host", "")).strip() == ""
            ):
                del node["controlPlaneEndpoint"]
                changed = True

        if node.get("kind") == "MachineHealthCheck":
            if transform_machine_health_check(node):
                changed = True

        if node.get("kind") == "KubeadmControlPlane":
            spec = node.get("spec")
            if isinstance(spec, CommentedMap):
                mt = spec.get("machineTemplate")
                if isinstance(mt, CommentedMap):
                    mt_spec = mt.get("spec")
                    if not isinstance(mt_spec, CommentedMap):
                        mt_spec = CommentedMap()
                    if "infrastructureRef" in mt and "infrastructureRef" not in mt_spec:
                        mt_spec["infrastructureRef"] = mt.pop("infrastructureRef")
                        changed = True
                    if mt_spec:
                        mt["spec"] = mt_spec

        for value in node.values():
            if transform(value):
                changed = True

    elif isinstance(node, CommentedSeq):
        for item in node:
            if transform(item):
                changed = True

    return changed


def process_file(path: Path) -> bool:
    yaml = YAML()
    yaml.preserve_quotes = True
    yaml.width = 4096  # avoid reflowing long TLS cipher suite lines

    original_text = path.read_text(encoding="utf-8")
    updated_text = original_text.replace(
        "cluster.x-k8s.io/v1beta1", "cluster.x-k8s.io/v1beta2"
    )

    docs = list(yaml.load_all(updated_text))

    any_changed = updated_text != original_text
    for doc in docs:
        if transform(doc):
            any_changed = True

    if any_changed:
        with path.open("w", encoding="utf-8") as fh:
            yaml.dump_all(docs, fh)

    return any_changed


def main() -> int:
    parser = argparse.ArgumentParser(
        usage=usage, description="Convert kubeletExtraArgs maps to name/value lists."
    )
    parser.add_argument("yaml_file", type=Path, help="Path to the YAML file to update.")
    args = parser.parse_args()

    if not args.yaml_file.is_file():
        parser.error(f"{args.yaml_file} is not a file")

    changed = process_file(args.yaml_file)
    if changed:
        print(f"Updated: {args.yaml_file}")
    else:
        print(f"No changes needed: {args.yaml_file}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
