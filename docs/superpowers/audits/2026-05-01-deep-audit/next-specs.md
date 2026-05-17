# Downstream specs — derived from 2026-05-01 deep audit

Each cluster below should become its own brainstorm → spec → plan cycle.
Findings linked by `F-NNN` map to entries in `../2026-05-01-deep-audit.md`.

---

## Spec 1: Observability (priority: P1)

**Scope.** Make the backend operationally observable: metrics on every external call, traces correlated by request ID, real-dependency depth in `/readyz`, structured-log correlation across services. Mobile-side: ship logs/breadcrumbs back to a sink so end-user bug reports include a session trace.

**Findings covered:** F-029 (no metrics/traces), F-013 (foundation laid), F-011 (readyz extension to RabbitMQ/MQTT/HA), F-014 (replace WS query token — also touches log redaction).

**Why this first.** Every later spec — fixing performance, debugging a bug report, validating a refactor — needs operator-facing signals to confirm the fix worked. Today nothing tells us if a deployment is healthy beyond "did it crash". This unlocks the rest.

**Estimated effort.** 1–2 weeks. New `/metrics` endpoint via `prometheus/client_golang`; per-subsystem counters + histograms; OTel propagation via `traceparent`; readiness depth extended to MQTT + HA when present; a docker-compose Prometheus + Grafana for local visibility (optional).

---

## Spec 2: Test coverage hardening (priority: P1)

**Scope.** Drive every Tier 1 + Tier 2 package above 60% line coverage. Isolate the 55s `hass` test slowness so every other package's feedback loop stays sub-second. Add HTTP-handler-level tests (`httptest.NewRecorder` + chi router) for every domain handler.

**Findings covered:** F-006 (26.1% total), F-023 (hass 55s slowness), the implicit-coverage-gap subset of every Tier 1+2 finding.

**Why this matters.** Refactoring is gated on confidence. Today, modifying `internal/scene/service.go` carries hidden risk because handler-level error paths aren't covered. Every fix in this audit was validated by manual test runs — that doesn't scale.

**Estimated effort.** 1 week. Per-package: 5–10 tests on the highest-risk methods (Refresh, Logout, Run, parseTopics, Notification.OnSensorReading, Schedule.Current). Include a coverage gate in CI (`-covermode=atomic` + threshold check).

---

## Spec 3: WebSocket auth + contract versioning (priority: P2)

**Scope.** Replace the `?access_token=<JWT>` query-string authentication for WebSocket upgrades with a single-use ticket flow. Tighten `CheckOrigin` to the CORS allow-list. Add a `version` field to every `realtime.Event` and require mobile to tolerate unknown fields (it already does).

**Findings covered:** F-014 (token in query string leaks via reverse-proxy logs), F-021 (CheckOrigin returns true), F-022 (no version on event schema).

**Why now.** F-014 is a known anti-pattern — every additional log sink that captures URLs (CloudWatch, Loki, GitHub-rendered web inspector) becomes a token-leak vector. Versioning is a contract-hygiene cost that grows with codebase age — better cheap now than expensive later.

**Estimated effort.** 3–5 days. Backend: `POST /api/v1/ws/ticket` returns 60s single-use ticket bound to userID; WS handler accepts `?ticket=...` only. Mobile: call ticket endpoint immediately before WS upgrade. Event struct: add `Version int` (default 1).

---

## Spec 4 (deferred): Per-classroom configurability

**Scope.** Move alert thresholds and timezone from a global Rules struct to per-classroom configuration in DB, with admin UI to override defaults.

**Findings covered:** F-016 (global thresholds), F-017 (server timezone vs classroom timezone).

**Why deferred.** Single-tenant pet project doesn't feel the pain yet. Would jump to P1 the moment a second school joins.

**Estimated effort.** 1 week. New `classroom_settings` table; UI form; service reads per-classroom overrides on alert evaluation.

---

## Spec 5 (deferred): Mobile push delivery

**Scope.** Activate the FCM stub (Firebase project setup, plist/json wiring, token registration call to backend, foreground/background handlers, local notification rendering).

**Findings covered:** F-024 (FCM is a stub).

**Why deferred.** Requires creating a real Firebase project (Google account, billing, plist signing). Out of pure-code scope for a pet project.

**Estimated effort.** 2–3 days of code once Firebase is set up.

---

## Quick-wins backlog (no spec needed — bundle into a single PR)

- F-016 / F-017 / F-021 / F-028 — small hardening items each independently small.
- Document mosquitto's `allow_anonymous true` as dev-only in README.
- Add per-driver JSON-schema validation for `device.config` (F-019 follow-up).

---

## Tracking
Each downstream spec should:
1. Run through `superpowers:brainstorming` to refine scope.
2. Get its own design doc in `docs/superpowers/specs/<date>-<topic>-design.md`.
3. Become its own implementation plan in `docs/superpowers/plans/<date>-<topic>.md`.
4. Reference the source audit findings (`F-NNN`) so traceability stays live across specs.
