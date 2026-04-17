# Smart Classroom

Backend for smart-classroom management: auth, devices (Tuya/Shelly/Sonoff/Aqara/PJLink/HA/MQTT via abstraction), schedule, scenes, sensors, analytics, notifications, real-time updates.

## Stack

- **Backend:** Go 1.25, chi, pgx, goose, JWT, zap, gorilla/websocket
- **Frontend (planned):** React 18 + Vite + TypeScript + Tailwind + shadcn/ui
- **Infra:** PostgreSQL 16, Docker Compose

## Run (Docker)

```bash
cp .env.example .env          # optional; compose has defaults
docker compose up --build
```

- Frontend (nginx) → http://localhost:3000
- Backend → http://localhost:8080
- Postgres → localhost:5432

The frontend proxies `/api/*` and `/api/v1/ws` to the backend, so the UI talks to a same-origin URL in production.

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
docker-compose.yml
```

## Phases

- [x] **Phase 1** — auth, user, JWT, Postgres, migrations, i18n, rate-limit, CORS, Docker, tests
- [x] **Phase 2** — classrooms, devices, devicectl Driver abstraction (generic HTTP), WebSocket hub/broker
- [x] **Phase 3** — schedule (weekly lessons + overlap + current), scenes (command sequences), sensors (ingestion + history + latest)
- [x] **Phase 4** — notifications (warning triggers: high/low temp, humidity, device offline), audit log (admin-only), analytics (sensor series, device usage, energy)
- [x] **Phase 5** — React 18 + Vite + TS + Tailwind + TanStack Query + Zustand + react-i18next (EN/RU/KZ). Mobile-width PWA shell with all screens wired to the backend. Served by nginx in Docker.
