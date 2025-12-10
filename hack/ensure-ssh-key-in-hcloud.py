#!/usr/bin/env python3
"""
Ensure that the given SSH key exists in Hetzner Cloud with the expected content.
If the key already exists and the public key matches, the script exits 0.
If the key exists but the content differs, the script fails.
Otherwise it creates the key via the Hetzner API.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def env_or_exit(name: str, example: str) -> str:
    value = os.environ.get(name)
    if not value:
        print(f"{name} not set. Example: {example}", file=sys.stderr)
        sys.exit(1)
    return value


def read_ssh_key(path: str) -> str:
    try:
        with open(path, "r", encoding="utf-8") as fp:
            return fp.read().strip()
    except OSError as err:
        print(f"failed to read SSH key {path}: {err}", file=sys.stderr)
        sys.exit(1)


def hetzner_request(
    url: str, token: str, method: str = "GET", payload: bytes | None = None
) -> dict[str, object]:
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    request = urllib.request.Request(url, data=payload, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request) as response:
            body = response.read()
    except urllib.error.HTTPError as err:
        body = err.read()
        print(body.decode("utf-8", errors="ignore"), file=sys.stderr)
        sys.exit(1)
    except urllib.error.URLError as err:
        print(f"failed to reach Hetzner API: {err}", file=sys.stderr)
        sys.exit(1)

    try:
        return json.loads(body)
    except json.JSONDecodeError as err:
        print(f"failed to parse Hetzner API response: {err}", file=sys.stderr)
        sys.exit(1)


def main() -> None:
    ssh_key_path = env_or_exit(
        "SSH_KEY", "path to a public key (e.g. $HOME/.ssh/shared-2024-07-08.pub)"
    )
    ssh_name = env_or_exit("SSH_KEY_NAME", "shared-2024-07-08")
    hcloud_token = env_or_exit("HCLOUD_TOKEN", "your Hetzner API token")

    public_key = read_ssh_key(ssh_key_path)
    encoded_name = urllib.parse.quote_plus(ssh_name)
    url = f"https://api.hetzner.cloud/v1/ssh_keys?name={encoded_name}"

    data = hetzner_request(url, hcloud_token)
    ssh_keys = data.get("ssh_keys", [])

    if ssh_keys:
        existing = ssh_keys[0].get("public_key", "").strip()
        if existing == public_key:
            print(f"ok: SSH key {ssh_name} already exists with identical content.")
            sys.exit(0)
        print(
            f"error: SSH key {ssh_name} already exists with different public key data.",
            file=sys.stderr,
        )
        sys.exit(1)

    payload = json.dumps(
        {"labels": {}, "name": ssh_name, "public_key": public_key}
    ).encode("utf-8")
    hetzner_request(
        "https://api.hetzner.cloud/v1/ssh_keys",
        hcloud_token,
        method="POST",
        payload=payload,
    )
    print("SSH key created.")


if __name__ == "__main__":
    main()
