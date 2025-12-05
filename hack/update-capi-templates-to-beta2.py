#!/usr/bin/env python3
"""
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
import sys
from pathlib import Path

try:
    from ruamel.yaml import YAML
    from ruamel.yaml.comments import CommentedMap, CommentedSeq
except ImportError as exc:  # pragma: no cover - runtime dependency guard
    sys.stderr.write(
        "ruamel.yaml is required for this script. Install with "
        "`pip install ruamel.yaml`.\n"
    )
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
    if any(hint in kind for hint in infra_hints) or kind.endswith("Cluster") or kind.endswith("MachineTemplate"):
        return "infrastructure.cluster.x-k8s.io"

    return None


def transform(node) -> bool:
    """
    Recursively walk the YAML structure, convert kubeletExtraArgs mappings,
    drop apiVersion inside infrastructureRef/controlPlaneRef/configRef blocks,
    and ensure apiGroup is set on those refs.

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
        description="Convert kubeletExtraArgs maps to name/value lists."
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
