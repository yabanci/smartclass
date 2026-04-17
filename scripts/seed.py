#!/usr/bin/env python3
"""Seed a fresh stack with demo credentials and sample data.

Creates:
    admin@smartclass.kz    / admin1234     (role: admin)
    teacher@smartclass.kz  / teacher1234   (role: teacher)

A classroom "Kabinet 101" owned by the teacher, plus three sample devices:
    - "Relay"         (generic_http   — fake REST endpoint)
    - "Kitchen Light" (homeassistant  — points to the bundled HA at :8123)
    - "Samsung AC"    (smartthings    — placeholder token, won't reach real cloud)

Safe to re-run: if a user already exists the script prints a notice and
continues. The admin's token is used for all writes.

Usage:
    make up
    make seed
"""

from __future__ import annotations

import json
import sys
import time
import urllib.error
import urllib.request

FRONTEND = "http://localhost:3000"

ADMIN = {"email": "admin@smartclass.kz", "password": "admin1234",
         "fullName": "Admin User", "role": "admin"}
TEACHER = {"email": "teacher@smartclass.kz", "password": "teacher1234",
           "fullName": "Teacher One", "role": "teacher"}


def req(method, path, *, token=None, body=None, expect=(200, 201, 204, 409)):
    url = FRONTEND + path
    data = json.dumps(body).encode() if body is not None else None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    r = urllib.request.Request(url, method=method, data=data, headers=headers)
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        raw = resp.read().decode()
        return resp.status, (json.loads(raw) if raw.strip() else {})
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        if e.code in expect:
            return e.code, (json.loads(raw) if raw.strip() else {})
        print(f"  ✗ {method} {path}: HTTP {e.code} — {raw[:200]}")
        sys.exit(1)


def wait_for_stack():
    for _ in range(60):
        try:
            urllib.request.urlopen(FRONTEND + "/", timeout=2)
            urllib.request.urlopen(FRONTEND + "/api/v1/../../healthz", timeout=2)
            return
        except Exception:
            time.sleep(1)
    print("stack not reachable at", FRONTEND)
    sys.exit(2)


def register_or_login(user: dict) -> tuple[str, str]:
    status, body = req("POST", "/api/v1/auth/register", body=user, expect=(201, 409))
    if status == 201:
        print(f"  ✓ registered {user['email']}")
        return body["data"]["user"]["id"], body["data"]["tokens"]["accessToken"]

    # already exists → login
    _, body = req("POST", "/api/v1/auth/login",
                  body={"email": user["email"], "password": user["password"]})
    print(f"  = logged in  {user['email']} (already existed)")
    return body["data"]["user"]["id"], body["data"]["tokens"]["accessToken"]


def main():
    print("→ waiting for stack at", FRONTEND)
    wait_for_stack()

    print("→ users")
    admin_id, admin_tok = register_or_login(ADMIN)
    teacher_id, _ = register_or_login(TEACHER)

    print("→ classroom")
    _, body = req("GET", "/api/v1/classrooms", token=admin_tok)
    existing = next((c for c in body["data"] if c["name"] == "Kabinet 101"), None)
    if existing:
        classroom_id = existing["id"]
        print(f"  = classroom already exists: {classroom_id}")
    else:
        _, body = req("POST", "/api/v1/classrooms", token=admin_tok,
                      body={"name": "Kabinet 101",
                            "description": "Sample classroom with mixed devices"})
        classroom_id = body["data"]["id"]
        print(f"  ✓ created classroom {classroom_id}")

    _, body = req("POST", f"/api/v1/classrooms/{classroom_id}/members",
                  token=admin_tok,
                  body={"userId": teacher_id},
                  expect=(204, 409))
    print("  ✓ assigned teacher to classroom")

    print("→ devices")
    _, body = req("GET", f"/api/v1/classrooms/{classroom_id}/devices", token=admin_tok)
    existing_names = {d["name"] for d in body["data"]}

    samples = [
        {
            "name": "Relay",
            "type": "relay",
            "brand": "generic",
            "driver": "generic_http",
            "config": {
                "baseUrl": "http://host.docker.internal:18081",
                "commands": {
                    "ON":  {"method": "GET", "path": "/anything?on"},
                    "OFF": {"method": "GET", "path": "/anything?off"},
                },
            },
        },
        {
            "name": "Kitchen Light",
            "type": "light",
            "brand": "xiaomi",
            "driver": "homeassistant",
            "config": {
                "baseUrl": "http://homeassistant:8123",
                "token": "REPLACE-with-long-lived-token-from-HA",
                "entityId": "light.kitchen",
            },
        },
        {
            "name": "Samsung AC",
            "type": "climate",
            "brand": "samsung",
            "driver": "smartthings",
            "config": {
                "token": "REPLACE-with-PAT-from-account.smartthings.com",
                "deviceId": "REPLACE-with-device-uuid",
            },
        },
    ]

    for s in samples:
        if s["name"] in existing_names:
            print(f"  = device '{s['name']}' already exists")
            continue
        req("POST", "/api/v1/devices", token=admin_tok,
            body={**s, "classroomId": classroom_id})
        print(f"  ✓ created device '{s['name']}'")

    print("→ schedule sample")
    _, body = req("GET", f"/api/v1/classrooms/{classroom_id}/schedule", token=admin_tok)
    if not any(body["data"].get(str(d)) for d in (1, 2, 3, 4, 5)):
        for day, subj, s, e in [
            (1, "Math",       "09:00", "09:45"),
            (1, "Physics",    "10:00", "10:45"),
            (2, "Literature", "09:00", "09:45"),
            (3, "History",    "09:00", "09:45"),
        ]:
            req("POST", "/api/v1/schedule", token=admin_tok, body={
                "classroomId": classroom_id, "subject": subj,
                "dayOfWeek": day, "startsAt": s, "endsAt": e,
            })
        print("  ✓ created 4 sample lessons")
    else:
        print("  = schedule already populated")

    print()
    print("Done. Login at http://localhost:3000 with:")
    print("  Admin:   admin@smartclass.kz    / admin1234")
    print("  Teacher: teacher@smartclass.kz  / teacher1234")


if __name__ == "__main__":
    main()
