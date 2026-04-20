#!/usr/bin/env python3
"""Poll the stack until it is fully ready.

Waits for:
    1. Backend /healthz to answer 200
    2. Seed endpoints are reachable (admin user exists — creates it via
       auto-seed if missing by calling `make seed` once)
    3. HA /api/v1/hass/selftest reports ok=true

Runs up to 5 minutes. On timeout prints the last seen self-check output so
the failure is actionable. Intended for `make wait` so a user can type
`make up && make wait && make verify` and be done.
"""

from __future__ import annotations

import json
import subprocess
import sys
import time
import urllib.error
import urllib.request

API = "http://localhost:8080"
TIMEOUT_S = 300
POLL_S = 5


def ping_healthz() -> bool:
    try:
        with urllib.request.urlopen(API + "/healthz", timeout=5) as r:
            return r.status == 200
    except Exception:
        return False


def login() -> str | None:
    try:
        req = urllib.request.Request(
            API + "/api/v1/auth/login",
            data=json.dumps(
                {"email": "admin@smartclass.kz", "password": "admin1234"}
            ).encode(),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=10) as r:
            return json.loads(r.read())["data"]["accessToken"]
    except Exception:
        return None


def selftest(token: str) -> dict | None:
    try:
        req = urllib.request.Request(
            API + "/api/v1/hass/selftest",
            headers={"Authorization": f"Bearer {token}"},
        )
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.loads(r.read())
    except Exception:
        return None


def print_result(result: dict) -> None:
    overall = "OK" if result.get("ok") else "FAIL"
    print(f"\n=== self-check: {overall} ===")
    for c in result.get("checks", []):
        mark = "[ok]  " if c.get("ok") else "[FAIL]"
        print(f"{mark} {c.get('name', '?').ljust(26)} {c.get('message', '')}")


def main() -> int:
    deadline = time.time() + TIMEOUT_S
    seeded = False
    last: dict | None = None
    print(f"Waiting up to {TIMEOUT_S}s for the stack to come up...")

    while time.time() < deadline:
        if not ping_healthz():
            print(".", end="", flush=True)
            time.sleep(POLL_S)
            continue

        token = login()
        if not token and not seeded:
            print("\nSeeding demo users (make seed)...")
            try:
                subprocess.run(["python3", "scripts/seed.py"], check=False)
            except FileNotFoundError:
                pass
            seeded = True
            time.sleep(POLL_S)
            continue
        if not token:
            print("x", end="", flush=True)
            time.sleep(POLL_S)
            continue

        last = selftest(token)
        if last and last.get("ok"):
            print_result(last)
            print("\nStack is ready: http://localhost:3000")
            return 0

        print("!", end="", flush=True)
        time.sleep(POLL_S)

    print("\nTimed out waiting for the stack.")
    if last:
        print_result(last)
    else:
        print("No self-check response was received.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
