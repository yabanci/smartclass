# Smart Classroom

Backend for smart-classroom management: auth, devices (Tuya/Shelly/Sonoff/Aqara/PJLink/HA/MQTT via abstraction), schedule, scenes, sensors, analytics, notifications, real-time updates.

## Stack

- **Backend:** Go 1.25, chi, pgx, goose, JWT, zap, gorilla/websocket
- **Mobile:** Flutter 3.41.7 — Android + iOS, Riverpod, go_router, flutter_secure_storage
- **Infra:** PostgreSQL 16, Docker Compose, MQTT (Mosquitto), Home Assistant

## Quickstart

```bash
make up         # boots all services, waits for healthy
make seed       # creates demo users + classroom + 3 devices + weekly schedule (idempotent)
make wait       # polls the stack until backend + HA self-check are green (up to 5 min)
make verify     # one-shot readiness report (exit 0 = OK, 1 = something's red)
```

`make verify` hits `GET /api/v1/hass/selftest` (admin-auth) and prints a per-check table. Every failing check has a short message telling you where to look (credentials, onboarding, flow_handlers, xiaomi_home install, xiaomi_home startflow decode). The backend also logs one `hass: READY — …` or `hass: DEGRADED — …` banner automatically on every bootstrap so `docker logs smartclass-backend | grep hass:` answers the same question without curl.

Use the mobile app (or `curl`) to sign in with seeded credentials:

| Role    | Email                   | Password      |
|---------|-------------------------|---------------|
| admin   | `admin@smartclass.kz`   | `admin1234`   |
| teacher | `teacher@smartclass.kz` | `teacher1234` |

The teacher account has a pre-seeded **Kabinet 101** with three demo devices (one per driver: `generic_http`, `homeassistant`, `smartthings`) and a week of sample lessons.

### Day-to-day

```bash
make logs       # tail all services
make ps         # status + ports
make down       # stop (data preserved)
make clean      # stop + wipe volumes (Postgres, HA config, MQTT state)
make test       # Go unit + race tests (18 packages)
make e2e        # live 32-step API/websocket test (stack must be running)
```

## Local observability

The backend exposes Prometheus metrics at `GET /metrics`. To scrape them
locally:

```bash
docker run --rm -p 9090:9090 \
  -v "$PWD/docs/observability/prometheus.yml:/etc/prometheus/prometheus.yml" \
  prom/prometheus
```

Then open <http://localhost:9090> and try queries like:

- `sum(rate(cctv_smartclass_http_requests_total[1m])) by (status)`
- `histogram_quantile(0.95, sum(rate(cctv_smartclass_http_request_duration_seconds_bucket[5m])) by (le, route))`
- `increase(cctv_smartclass_auth_replay_detected_total[1h])`

For graphs: `docs/observability/dashboard.json` is a Grafana dashboard
you can import (Grafana → Dashboards → Import → Upload JSON).

`/healthz` is liveness (always 200 if the process is alive).
`/readyz` is readiness — it returns 503 + a per-check JSON breakdown
when any dependency (Postgres, Home Assistant) is unreachable.

## What's running

4 services, 1 network, 1 command:

| Service | URL / port | Purpose |
|---|---|---|
| **backend**  | http://localhost:8080 | Go API + WebSocket hub |
| **postgres** | localhost:5432 | App data + audit log |
| **homeassistant** | http://localhost:8123 | Universal device translator (Xiaomi / Samsung / Tuya / Aqara / Zigbee / Matter / …) |
| **mosquitto** | mqtt://localhost:1883 · ws://localhost:9001 | MQTT broker (Tasmota, Zigbee2MQTT, generic IoT) |

The UI is the Flutter mobile app. Run it with `flutter run -t lib/main_dev.dart` from `mobile/` — it connects to `http://localhost:8080/api/v1` by default.

### Home Assistant: now driven from our UI

You **do not need to open `http://localhost:8123` anymore**. On a fresh `make up` the backend automatically:

1. Polls HA until it's reachable, then calls its onboarding API to create the owner account (`smartclass` / `smartclass1234` by default — override with `HASS_OWNER_USERNAME` / `HASS_OWNER_PASSWORD` env vars).
2. Mints a long-lived access token and persists it in our `hass_config` Postgres table.
3. Exposes the rest through our own REST API and a wizard on the **Devices** page (button "Найти IoT" / "Find IoT"):
   - `GET /api/v1/hass/integrations` — list every integration HA knows about (Xiaomi, Tuya, MQTT, Zigbee2MQTT, SmartThings, …).
   - `POST /api/v1/hass/flows` + `POST /api/v1/hass/flows/{id}/step` — proxy HA's dynamic config flow so the user steps through pairing inside our UI.
   - `GET /api/v1/hass/entities` — discovered devices, ready to be adopted.
   - `POST /api/v1/hass/adopt` — turns an HA `entity_id` into a Device row in a classroom (driver `homeassistant`, config pre-filled with the shared token).

If your `hass-config` volume already contains a manually-onboarded HA install (returns `409 hass_already_onboarded`), open `http://localhost:8123 → Profile → Security → Long-Lived Access Tokens → Create Token` once, paste it into the wizard's "Save token" form, and from then on everything stays in our UI. To re-trigger auto-onboarding from scratch run `make clean && make up`.

## Dev (Flutter mobile)

```bash
cd mobile
flutter pub get
flutter run                        # dev flavor — connects to localhost:8080
# or
flutter run -t lib/main_dev.dart   # explicit dev entry
flutter run -t lib/main_prod.dart  # prod entry (api.smartclass.kz)

flutter test                       # unit tests
flutter analyze                    # static analysis
flutter build apk --release        # release APK (debug-signed)
```

> iOS build requires Xcode + Apple Developer account. Run `flutter run` on a connected iPhone or `flutter build ipa --release` with a valid signing identity.

## Dev (local Go)

```bash
cd backend
cp .env.example .env
go mod tidy
make test
make run
```

## Layout

```
backend/
  cmd/server/               entrypoint
  internal/
    auth/                   register/login/refresh/jwt
    user/                   user entity + profile
    platform/               shared infra (db, httpx, hasher, i18n, validator, middleware)
    server/                 http server wiring + routes
  migrations/               SQL migrations (goose)
  locales/                  i18n bundles
mobile/                     Flutter 3.41.7 — Android/iOS
  lib/
    config/                 AppConfig (dev/prod flavors)
    core/                   api client, router, storage, i18n, WS, push
    features/               auth, home, devices, schedule, scenes, analytics, …
    shared/                 models, providers (Riverpod), widgets
  test/                     unit tests (59+)
  integration_test/         end-to-end flow
docker-compose.yml
Makefile
```

## Phases

- [x] **Phase 1** — auth, user, JWT, Postgres, migrations, i18n, rate-limit, CORS, Docker, tests
- [x] **Phase 2** — classrooms, devices, devicectl Driver abstraction (generic HTTP), WebSocket hub/broker
- [x] **Phase 3** — schedule (weekly lessons + overlap + current), scenes (command sequences), sensors (ingestion + history + latest)
- [x] **Phase 4** — notifications (warning triggers: high/low temp, humidity, device offline), audit log (admin-only), analytics (sensor series, device usage, energy)
- [x] **Phase 5** — React 18 SPA (removed; superseded by Flutter mobile).
- [x] **Phase 6** — Flutter 3.41.7 native mobile app (Android/iOS). Riverpod state management, go_router navigation, flutter_secure_storage (Keychain/EncryptedSharedPreferences), WebSocket real-time updates, offline banner, i18n EN/RU/KK, dev/prod flavors, FCM stub. GitHub Actions CI: analyze + test + release APK.

## Device drivers

The backend ships three drivers out of the box. Each one plugs into the same `devicectl.Driver` interface — adding another protocol is one file under `backend/internal/devicectl/drivers/<name>/` plus a `factory.Register(...)` line in `cmd/server/main.go`.

| Driver | Name in API | When to use |
|---|---|---|
| `generic_http` | `generic_http` | Shelly Gen1 / Sonoff DIY / Tasmota / any LAN device with a plain REST API |
| `homeassistant` | `homeassistant` | Recommended for **Xiaomi Mi Home, Samsung SmartThings, Tuya, Aqara, Sonoff, Matter, Zigbee, HomeKit** — Home Assistant acts as a universal translator and we hit its REST API |
| `smartthings` | `smartthings` | Official Samsung SmartThings REST v1 (no HA needed) |

### Home Assistant (covers Xiaomi, Samsung, Huawei-via-Matter, Tuya, Aqara, Sonoff)

1. Run Home Assistant (Home Assistant OS, Supervised, Container) and pair your devices through its UI (`Settings → Devices & Services`).
2. Generate a **long-lived access token**: `user profile → Long-lived Access Tokens → Create token`.
3. Register the device in our backend:

```bash
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{
    "classroomId": "<uuid>",
    "name": "Kitchen Light",
    "type": "light",
    "brand": "xiaomi",
    "driver": "homeassistant",
    "config": {
      "baseUrl": "http://homeassistant.local:8123",
      "token": "<long-lived token>",
      "entityId": "light.kitchen"
    }
  }'
```

Supported commands per HA domain:
- `switch.*`, `light.*` → `ON`, `OFF`, `SET_VALUE` (light: brightness 0-100)
- `cover.*` → `OPEN`, `CLOSE`, `SET_VALUE` (position 0-100)
- `lock.*` → `OPEN`/`CLOSE` (unlock/lock)
- `climate.*` → `SET_VALUE` (target temperature)
- `fan.*` → `SET_VALUE` (percentage)

### SmartThings (Samsung)

1. Get a **Personal Access Token** at https://account.smartthings.com/tokens.
2. List devices to get their UUIDs: `curl -H "Authorization: Bearer $PAT" https://api.smartthings.com/v1/devices`.
3. Register:

```bash
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{
    "classroomId": "<uuid>",
    "name": "Samsung AC",
    "type": "climate",
    "brand": "samsung",
    "driver": "smartthings",
    "config": {
      "token": "pat-...",
      "deviceId": "<smartthings device uuid>"
    }
  }'
```

By default the driver maps `ON/OFF → switch`, `OPEN/CLOSE → windowShade`, `SET_VALUE → switchLevel.setLevel`. Override via `"capability"` and `"setCommand"` for locks (`"lock"` + `unlock/lock`), thermostats (`"thermostatHeatingSetpoint"` + `"setHeatingSetpoint"`), coloured bulbs, etc.

### Notes on specific vendors

- **Xiaomi / Mi Home / Aqara** — no stable public API. Go through Home Assistant. We ship Xiaomi's official `xiaomi_home` integration pre-installed (OAuth via Mi account), plus legacy fallbacks (`xiaomi_miio`, `aqara`, `xiaomi_aqara`).
    - **One-time host-file entry required.** Xiaomi's OAuth only accepts `http://homeassistant.local:8123` as a redirect target. If HA is running inside Docker on your workstation, point that hostname at 127.0.0.1:
        - **Windows:** open `C:\Windows\System32\drivers\etc\hosts` in Notepad as Administrator → add line `127.0.0.1 homeassistant.local` → save.
        - **macOS / Linux:** `echo "127.0.0.1 homeassistant.local" | sudo tee -a /etc/hosts`.
    - After adding the entry, open HA at `http://homeassistant.local:8123` in the same browser you'll use for the OAuth wizard — if that page loads, the redirect will work.
- **Huawei (HarmonyOS / HiLink)** — no public API. The only reliable path is buying Matter-certified Huawei devices and pairing them via Home Assistant's Matter integration, then using the `homeassistant` driver.
- **Samsung** — either `smartthings` directly, or via Home Assistant.
- **Tuya / Smart Life** — Home Assistant has an official Tuya integration (cloud).
- **Sonoff / Shelly** — `generic_http` works on LAN for most models; otherwise Home Assistant.
