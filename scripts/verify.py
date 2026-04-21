#!/usr/bin/env python3
"""One-shot smart-classroom readiness check.

Logs in as admin, calls the backend's live HA self-check, and prints a
human-readable table. Exits 0 when everything is green, 1 when any check is
red — suitable for CI or a `make verify` gate.

Usage:
    make verify
"""

from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request

API = "http://localhost:8080/api/v1"


def post_json(path: str, body: dict, token: str | None = None) -> dict:
    req = urllib.request.Request(
        API + path,
        data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())


def get_json(path: str, token: str) -> dict:
    req = urllib.request.Request(
        API + path, headers={"Authorization": f"Bearer {token}"}
    )
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())


def login() -> str:
    try:
        data = post_json(
            "/auth/login",
            {"email": "admin@smartclass.kz", "password": "admin1234"},
        )
        return data["data"]["tokens"]["accessToken"]
    except urllib.error.URLError as e:
        sys.stderr.write(
            f"cannot reach backend at {API}: {e}\n"
            "Run `make up` first, then `make seed` (to create admin@smartclass.kz).\n"
        )
        sys.exit(2)


def main() -> int:
    token = login()
    try:
        result = get_json("/hass/selftest", token)
    except urllib.error.HTTPError as e:
        sys.stderr.write(f"/hass/selftest HTTP {e.code}: {e.read().decode()}\n")
        return 2

    overall = "OK " if result.get("ok") else "FAIL"
    print(f"smart-classroom self-check: {overall}")
    print("-" * 72)
    for c in result.get("checks", []):
        mark = "[ok]  " if c.get("ok") else "[FAIL]"
        name = c.get("name", "?").ljust(26)
        print(f"{mark} {name} {c.get('message', '')}")
    print("-" * 72)
    return 0 if result.get("ok") else 1


if __name__ == "__main__":
    sys.exit(main())
