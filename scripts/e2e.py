#!/usr/bin/env python3
"""End-to-end test for the Smart Classroom stack.

Brings up a fake Home Assistant + fake SmartThings on loopback so the
backend's drivers actually get exercised. Walks every public API end-to-end
through the frontend nginx proxy (http://localhost:3000/api/v1/*) — proving
that nginx → backend → (Postgres, HA, ST, MQTT, WebSocket) works.

Requires the stack to be running: `make up`.

Usage:
    python3 scripts/e2e.py
    FRONTEND=http://host:3000 python3 scripts/e2e.py
"""

from __future__ import annotations

import asyncio
import json
import os
import random
import string
import sys
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlencode

try:
    import websockets  # type: ignore
except ImportError:
    websockets = None  # WS step is skipped if unavailable

FRONTEND = os.environ.get("FRONTEND", "http://localhost:3000")
BACKEND = os.environ.get("BACKEND", "http://localhost:8080")
HOST_ALIAS = os.environ.get("HOST_ALIAS", "host.docker.internal")

PASSED: list[str] = []
FAILED: list[tuple[str, str]] = []


def step(name: str):
    def deco(fn):
        def wrapper(*a, **kw):
            try:
                result = fn(*a, **kw)
                PASSED.append(name)
                sys.stdout.write(f"  ✓ {name}\n")
                sys.stdout.flush()
                return result
            except AssertionError as e:
                FAILED.append((name, str(e)))
                sys.stdout.write(f"  ✗ {name}: {e}\n")
                sys.stdout.flush()
                raise
            except Exception as e:
                FAILED.append((name, repr(e)))
                sys.stdout.write(f"  ✗ {name}: {e!r}\n")
                sys.stdout.flush()
                raise

        return wrapper

    return deco


def rand(n: int = 6) -> str:
    return "".join(random.choices(string.ascii_lowercase + string.digits, k=n))


# ---------- fake IoT servers ----------


class FakeHA(BaseHTTPRequestHandler):
    calls: list[tuple[str, str, dict]] = []

    def _json(self, status: int, body):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def do_POST(self):  # noqa: N802
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length)) if length else {}
        FakeHA.calls.append(("POST", self.path, body))
        self._json(200, [{"entity_id": body.get("entity_id"), "state": "on"}])

    def do_GET(self):  # noqa: N802
        FakeHA.calls.append(("GET", self.path, {}))
        self._json(200, {"state": "on", "attributes": {"friendly_name": "Fake"}})

    def log_message(self, *args):  # silence
        pass


class FakeST(BaseHTTPRequestHandler):
    calls: list[tuple[str, str, dict]] = []

    def _json(self, status: int, body):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def do_POST(self):  # noqa: N802
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length)) if length else {}
        FakeST.calls.append(("POST", self.path, body))
        self._json(200, {"results": [{"id": "cmd1", "status": "ACCEPTED"}]})

    def do_GET(self):  # noqa: N802
        FakeST.calls.append(("GET", self.path, {}))
        self._json(
            200,
            {
                "components": {
                    "main": {"switch": {"switch": {"value": "on"}}},
                }
            },
        )

    def log_message(self, *args):
        pass


def serve(handler_cls, port: int) -> HTTPServer:
    srv = HTTPServer(("0.0.0.0", port), handler_cls)
    th = threading.Thread(target=srv.serve_forever, daemon=True)
    th.start()
    return srv


# ---------- HTTP client ----------


class ApiError(Exception):
    def __init__(self, code: int, body: str):
        super().__init__(f"HTTP {code}: {body}")
        self.code = code
        self.body = body


def req(
    method: str,
    path: str,
    *,
    token: str | None = None,
    json_body: dict | None = None,
    base: str = FRONTEND,
    expect: int | None = None,
    lang: str | None = None,
) -> dict:
    url = base + path
    data = json.dumps(json_body).encode() if json_body is not None else None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if lang:
        headers["Accept-Language"] = lang
    r = urllib.request.Request(url, method=method, data=data, headers=headers)
    try:
        resp = urllib.request.urlopen(r, timeout=15)
        body = resp.read().decode() or "{}"
        if expect is not None and resp.status != expect:
            raise AssertionError(f"expected {expect}, got {resp.status}: {body[:200]}")
        return json.loads(body) if body.strip() else {}
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        if expect is not None and e.code == expect:
            return json.loads(body) if body.strip() else {}
        raise ApiError(e.code, body)


def data(env: dict):
    assert "data" in env, f"no data in {env}"
    return env["data"]


# ---------- fixtures ----------


class Ctx:
    admin_token: str = ""
    teacher_token: str = ""
    teacher_id: str = ""
    classroom_id: str = ""
    device_http_id: str = ""
    device_ha_id: str = ""
    device_st_id: str = ""
    scene_id: str = ""
    lesson_id: str = ""


ctx = Ctx()


# ---------- test steps ----------


@step("health: backend /healthz (direct)")
def t_backend_health():
    r = urllib.request.urlopen(BACKEND + "/healthz", timeout=5)
    assert r.status == 200


@step("health: nginx serves index.html")
def t_frontend_root():
    r = urllib.request.urlopen(FRONTEND + "/", timeout=5)
    html = r.read().decode()
    assert r.status == 200 and "<title>Smart Classroom</title>" in html


@step("auth: register teacher via /api proxy")
def t_register_teacher():
    suf = rand()
    env = req(
        "POST",
        "/api/v1/auth/register",
        json_body={
            "email": f"t-{suf}@ex.com",
            "password": "password1",
            "fullName": f"Teacher {suf}",
            "role": "teacher",
        },
    )
    d = data(env)
    ctx.teacher_token = d["tokens"]["accessToken"]
    ctx.teacher_id = d["user"]["id"]
    assert d["user"]["role"] == "teacher"


@step("auth: register admin (for /logs later)")
def t_register_admin():
    suf = rand()
    env = req(
        "POST",
        "/api/v1/auth/register",
        json_body={
            "email": f"a-{suf}@ex.com",
            "password": "password1",
            "fullName": f"Admin {suf}",
            "role": "admin",
        },
    )
    ctx.admin_token = data(env)["tokens"]["accessToken"]


@step("auth: login flow returns new tokens")
def t_login():
    # re-login the teacher we just made
    me = data(req("GET", "/api/v1/users/me", token=ctx.teacher_token))
    env = req(
        "POST", "/api/v1/auth/login",
        json_body={"email": me["email"], "password": "password1"},
    )
    assert data(env)["tokens"]["accessToken"]


@step("auth: refresh with refreshToken returns access")
def t_refresh():
    suf = rand()
    reg = data(req("POST", "/api/v1/auth/register", json_body={
        "email": f"r-{suf}@ex.com", "password": "password1",
        "fullName": "R User", "role": "teacher",
    }))
    refresh = reg["tokens"]["refreshToken"]
    env = req("POST", "/api/v1/auth/refresh", json_body={"refreshToken": refresh})
    assert data(env)["tokens"]["accessToken"]


@step("auth: rejects wrong password (localized to RU)")
def t_login_bad():
    me = data(req("GET", "/api/v1/users/me", token=ctx.teacher_token))
    try:
        req("POST", "/api/v1/auth/login",
            json_body={"email": me["email"], "password": "nope-nope"},
            lang="ru")
        raise AssertionError("should have failed")
    except ApiError as e:
        assert e.code == 401
        assert "Неверный" in e.body, f"expected RU error, got {e.body}"


@step("user: PATCH /me updates profile")
def t_update_profile():
    env = req("PATCH", "/api/v1/users/me",
              token=ctx.teacher_token,
              json_body={"fullName": "Updated Name", "language": "ru"})
    d = data(env)
    assert d["fullName"] == "Updated Name" and d["language"] == "ru"


@step("user: change password, login with new, restore")
def t_change_pw():
    req("POST", "/api/v1/users/me/password",
        token=ctx.teacher_token,
        json_body={"currentPassword": "password1", "newPassword": "password2"})
    me = data(req("GET", "/api/v1/users/me", token=ctx.teacher_token))
    env = req("POST", "/api/v1/auth/login",
              json_body={"email": me["email"], "password": "password2"})
    # restore so the rest of the tests work
    new_tok = data(env)["tokens"]["accessToken"]
    req("POST", "/api/v1/users/me/password", token=new_tok,
        json_body={"currentPassword": "password2", "newPassword": "password1"})


@step("classroom: create, list, get, update, delete lifecycle")
def t_classroom_crud():
    a = data(req("POST", "/api/v1/classrooms",
                 token=ctx.teacher_token,
                 json_body={"name": "Lab A", "description": "primary lab"}))
    lst = data(req("GET", "/api/v1/classrooms", token=ctx.teacher_token))
    assert any(c["id"] == a["id"] for c in lst)
    got = data(req("GET", f"/api/v1/classrooms/{a['id']}", token=ctx.teacher_token))
    assert got["name"] == "Lab A"

    upd = data(req("PATCH", f"/api/v1/classrooms/{a['id']}",
                   token=ctx.teacher_token,
                   json_body={"name": "Lab A2"}))
    assert upd["name"] == "Lab A2"
    req("DELETE", f"/api/v1/classrooms/{a['id']}", token=ctx.teacher_token, expect=204)

    # Then create the canonical classroom we use for the rest of the tests
    ctx.classroom_id = data(req("POST", "/api/v1/classrooms",
                                token=ctx.teacher_token,
                                json_body={"name": "Main Lab"}))["id"]


@step("classroom: outsider cannot read another's classroom")
def t_classroom_rbac():
    suf = rand()
    other = data(req("POST", "/api/v1/auth/register", json_body={
        "email": f"o-{suf}@ex.com", "password": "password1",
        "fullName": "Other Teacher", "role": "teacher",
    }))["tokens"]["accessToken"]
    try:
        req("GET", f"/api/v1/classrooms/{ctx.classroom_id}", token=other)
        raise AssertionError("expected 403")
    except ApiError as e:
        assert e.code == 403


@step("device: create generic_http device (via httpbin)")
def t_device_generic():
    cfg = {
        "baseUrl": f"http://{HOST_ALIAS}:18081",
        "commands": {
            "ON":  {"method": "GET", "path": "/status/200"},
            "OFF": {"method": "GET", "path": "/status/200"},
        },
    }
    env = req("POST", "/api/v1/devices", token=ctx.teacher_token, json_body={
        "classroomId": ctx.classroom_id,
        "name": "Relay", "type": "relay", "brand": "generic",
        "driver": "generic_http", "config": cfg,
    })
    ctx.device_http_id = data(env)["id"]


@step("device: create homeassistant device (pointed at fake HA)")
def t_device_ha():
    env = req("POST", "/api/v1/devices", token=ctx.teacher_token, json_body={
        "classroomId": ctx.classroom_id,
        "name": "Kitchen Light", "type": "light", "brand": "xiaomi",
        "driver": "homeassistant",
        "config": {
            "baseUrl": f"http://{HOST_ALIAS}:18123",
            "token": "fake-ha-pat",
            "entityId": "light.kitchen",
        },
    })
    ctx.device_ha_id = data(env)["id"]


@step("device: create smartthings device (pointed at fake ST)")
def t_device_st():
    env = req("POST", "/api/v1/devices", token=ctx.teacher_token, json_body={
        "classroomId": ctx.classroom_id,
        "name": "Samsung AC", "type": "climate", "brand": "samsung",
        "driver": "smartthings",
        "config": {
            "token": "pat-fake",
            "deviceId": "device-uuid-1",
            "baseUrl": f"http://{HOST_ALIAS}:18124/v1",
        },
    })
    ctx.device_st_id = data(env)["id"]


@step("device: list by classroom includes all three")
def t_device_list():
    lst = data(req("GET", f"/api/v1/classrooms/{ctx.classroom_id}/devices",
                   token=ctx.teacher_token))
    ids = {d["id"] for d in lst}
    assert {ctx.device_http_id, ctx.device_ha_id, ctx.device_st_id} <= ids


@step("device: ON command on generic_http device")
def t_cmd_http():
    d = data(req("POST", f"/api/v1/devices/{ctx.device_http_id}/commands",
                 token=ctx.teacher_token,
                 json_body={"type": "ON"}))
    assert d["status"] == "on" and d["online"] is True


@step("device: ON command routed through homeassistant driver")
def t_cmd_ha():
    before = len(FakeHA.calls)
    d = data(req("POST", f"/api/v1/devices/{ctx.device_ha_id}/commands",
                 token=ctx.teacher_token,
                 json_body={"type": "ON"}))
    assert d["status"] == "on" and d["online"] is True
    new_calls = FakeHA.calls[before:]
    assert any(c[0] == "POST" and "/api/services/light/turn_on" in c[1] for c in new_calls), \
        f"fake HA did not receive expected call: {new_calls}"


@step("device: SET_VALUE on smartthings device")
def t_cmd_st():
    before = len(FakeST.calls)
    data(req("POST", f"/api/v1/devices/{ctx.device_st_id}/commands",
             token=ctx.teacher_token,
             json_body={"type": "SET_VALUE", "value": 70}))
    new_calls = FakeST.calls[before:]
    assert any(c[0] == "POST" and "/commands" in c[1]
               and c[2]["commands"][0]["command"] == "setLevel"
               for c in new_calls), f"fake ST missed call: {new_calls}"


@step("device: unsupported command returns 400")
def t_cmd_bad():
    try:
        req("POST", f"/api/v1/devices/{ctx.device_http_id}/commands",
            token=ctx.teacher_token, json_body={"type": "EXPLODE"})
        raise AssertionError("expected 400")
    except ApiError as e:
        assert e.code == 400


@step("schedule: create lesson, reject overlap")
def t_schedule():
    a = data(req("POST", "/api/v1/schedule", token=ctx.teacher_token, json_body={
        "classroomId": ctx.classroom_id, "subject": "Math",
        "dayOfWeek": 1, "startsAt": "09:00", "endsAt": "10:00",
    }))
    ctx.lesson_id = a["id"]
    try:
        req("POST", "/api/v1/schedule", token=ctx.teacher_token, json_body={
            "classroomId": ctx.classroom_id, "subject": "Physics",
            "dayOfWeek": 1, "startsAt": "09:30", "endsAt": "10:30",
        })
        raise AssertionError("expected overlap conflict")
    except ApiError as e:
        assert e.code == 409


@step("schedule: week view has 5 buckets")
def t_schedule_week():
    wk = data(req("GET", f"/api/v1/classrooms/{ctx.classroom_id}/schedule",
                  token=ctx.teacher_token))
    assert set(wk.keys()) == {"1", "2", "3", "4", "5"}
    assert len(wk["1"]) >= 1


@step("schedule: update + delete")
def t_schedule_update_delete():
    data(req("PATCH", f"/api/v1/schedule/{ctx.lesson_id}", token=ctx.teacher_token,
             json_body={"subject": "Algebra"}))
    req("DELETE", f"/api/v1/schedule/{ctx.lesson_id}",
        token=ctx.teacher_token, expect=204)


@step("scenes: create, run (all steps succeed)")
def t_scene():
    env = data(req("POST", "/api/v1/scenes", token=ctx.teacher_token, json_body={
        "classroomId": ctx.classroom_id,
        "name": "Lesson Mode",
        "description": "lights on, AC on",
        "steps": [
            {"deviceId": ctx.device_http_id, "command": "ON"},
            {"deviceId": ctx.device_ha_id, "command": "ON"},
        ],
    }))
    ctx.scene_id = env["id"]
    res = data(req("POST", f"/api/v1/scenes/{ctx.scene_id}/run",
                   token=ctx.teacher_token))
    assert res["steps"] and all(s["success"] for s in res["steps"]), res


@step("sensor: ingest batch, history, latest")
def t_sensors():
    env = data(req("POST", "/api/v1/sensors/readings",
                   token=ctx.teacher_token, json_body={
                       "readings": [
                           {"deviceId": ctx.device_http_id, "metric": "temperature", "value": 22.5, "unit": "C"},
                           {"deviceId": ctx.device_http_id, "metric": "humidity",    "value": 45.0, "unit": "%"},
                       ],
                   }))
    assert env["accepted"] == 2

    hist = data(req("GET",
                    f"/api/v1/devices/{ctx.device_http_id}/sensors/readings?metric=temperature",
                    token=ctx.teacher_token))
    assert len(hist) >= 1

    latest = data(req("GET",
                      f"/api/v1/classrooms/{ctx.classroom_id}/sensors/readings/latest",
                      token=ctx.teacher_token))
    metrics = {r["metric"] for r in latest}
    assert "temperature" in metrics and "humidity" in metrics


@step("notifications: high-temp reading triggers a warning")
def t_notif_trigger():
    before = data(req("GET", "/api/v1/notifications/unread-count",
                      token=ctx.teacher_token))["count"]
    req("POST", "/api/v1/sensors/readings", token=ctx.teacher_token, json_body={
        "readings": [
            {"deviceId": ctx.device_http_id, "metric": "temperature", "value": 35.5, "unit": "C"},
        ],
    })
    # give the trigger a tick
    time.sleep(0.2)
    after = data(req("GET", "/api/v1/notifications/unread-count",
                     token=ctx.teacher_token))["count"]
    assert after == before + 1, f"expected {before}+1 unread, got {after}"


@step("notifications: list, mark-read, mark-all-read")
def t_notif_manage():
    lst = data(req("GET", "/api/v1/notifications", token=ctx.teacher_token))
    assert len(lst) >= 1
    first = lst[0]
    assert first.get("readAt") is None
    req("POST", f"/api/v1/notifications/{first['id']}/read",
        token=ctx.teacher_token, expect=204)
    req("POST", "/api/v1/notifications/read-all",
        token=ctx.teacher_token, expect=204)
    unread = data(req("GET", "/api/v1/notifications/unread-count",
                      token=ctx.teacher_token))["count"]
    assert unread == 0


@step("analytics: sensor series, device usage, energy")
def t_analytics():
    env = req(
        "GET",
        f"/api/v1/classrooms/{ctx.classroom_id}/analytics/sensors?metric=temperature&bucket=hour",
        token=ctx.teacher_token)
    series = env.get("data")
    assert series is not None and len(series) >= 1, f"empty series: {env}"

    usage = data(req("GET",
                     f"/api/v1/classrooms/{ctx.classroom_id}/analytics/usage",
                     token=ctx.teacher_token))
    assert isinstance(usage, list)  # may be empty if audit not flushed yet

    energy = data(req("GET",
                      f"/api/v1/classrooms/{ctx.classroom_id}/analytics/energy",
                      token=ctx.teacher_token))
    assert "total" in energy


@step("analytics: invalid bucket rejected")
def t_analytics_bad():
    try:
        req("GET",
            f"/api/v1/classrooms/{ctx.classroom_id}/analytics/sensors?metric=temperature&bucket=week",
            token=ctx.teacher_token)
    except ApiError as e:
        raise AssertionError(f"week bucket should be valid: {e}")
    try:
        req("GET",
            f"/api/v1/classrooms/{ctx.classroom_id}/analytics/sensors?metric=temperature&bucket=millisecond",
            token=ctx.teacher_token)
        raise AssertionError("invalid bucket should 400")
    except ApiError as e:
        assert e.code == 400


@step("audit log: teacher gets 403, admin gets entries")
def t_logs():
    try:
        req("GET", "/api/v1/logs", token=ctx.teacher_token)
        raise AssertionError("teacher should 403")
    except ApiError as e:
        assert e.code == 403
    # audit is buffered 2s — wait a bit
    time.sleep(3)
    lst = data(req("GET", "/api/v1/logs?limit=50", token=ctx.admin_token))
    actions = {e["action"] for e in lst}
    assert {"create", "command"} & actions, f"expected create/command, got {actions}"


@step("websocket: connect, command, receive state_changed event")
def t_websocket():
    if websockets is None:
        raise AssertionError("python websockets pkg not installed; skip")

    async def run():
        url = (
            f"{FRONTEND.replace('http', 'ws')}/api/v1/ws"
            f"?access_token={ctx.teacher_token}"
            f"&{urlencode([('topic', f'classroom:{ctx.classroom_id}:devices')])}"
        )
        async with websockets.connect(url) as ws:
            # trigger a command → backend emits device.state_changed
            req("POST", f"/api/v1/devices/{ctx.device_http_id}/commands",
                token=ctx.teacher_token, json_body={"type": "OFF"})
            msg = await asyncio.wait_for(ws.recv(), timeout=3)
            evt = json.loads(msg)
            assert evt["type"].startswith("device."), evt

    asyncio.run(run())


@step("auth: no token → 401, bad token → 401")
def t_auth_paths():
    try:
        req("GET", "/api/v1/users/me")
        raise AssertionError("expected 401")
    except ApiError as e:
        assert e.code == 401
    try:
        req("GET", "/api/v1/users/me", token="not-a-real-jwt")
        raise AssertionError("expected 401")
    except ApiError as e:
        assert e.code == 401


@step("validation: bad body → 400 with field details")
def t_validation():
    try:
        req("POST", "/api/v1/auth/register", json_body={
            "email": "not-email", "password": "x", "fullName": "", "role": "pirate",
        })
        raise AssertionError("expected 400")
    except ApiError as e:
        body = json.loads(e.body)
        assert e.code == 400 and body["error"]["code"] == "validation_failed"
        assert isinstance(body["error"]["details"], list)


# ---------- main ----------


def main():
    ha = serve(FakeHA, 18123)
    st = serve(FakeST, 18124)
    hb = None
    # httpbin via docker only if available; otherwise use FakeHA-like 200-echo on 18081
    hb = serve(FakeHA, 18081)

    # wait for stack
    for _ in range(30):
        try:
            urllib.request.urlopen(BACKEND + "/healthz", timeout=2)
            break
        except Exception:
            time.sleep(1)
    else:
        print("backend not reachable at", BACKEND)
        sys.exit(2)

    steps = [
        t_backend_health, t_frontend_root,
        t_register_teacher, t_register_admin, t_login, t_refresh, t_login_bad,
        t_update_profile, t_change_pw,
        t_classroom_crud, t_classroom_rbac,
        t_device_generic, t_device_ha, t_device_st,
        t_device_list,
        t_cmd_http, t_cmd_ha, t_cmd_st, t_cmd_bad,
        t_schedule, t_schedule_week, t_schedule_update_delete,
        t_scene,
        t_sensors, t_notif_trigger, t_notif_manage,
        t_analytics, t_analytics_bad,
        t_logs,
        t_websocket,
        t_auth_paths, t_validation,
    ]
    for s in steps:
        try:
            s()
        except Exception:
            continue

    ha.shutdown(); st.shutdown()
    if hb: hb.shutdown()

    print(f"\nResults: {len(PASSED)} passed, {len(FAILED)} failed")
    for name, err in FAILED:
        print(f"  FAIL {name}: {err[:160]}")
    sys.exit(0 if not FAILED else 1)


if __name__ == "__main__":
    main()
