# Deep Audit — Design Spec

**Date:** 2026-05-01
**Topic:** Read-only deep audit of the smartclass repo, producing a prioritized findings doc that feeds downstream fix/improvement specs.
**Status:** Approved (ready for plan)

## 1. Purpose

User asked to "improve the project, fill gaps, fix bugs, deep analyze." The request spans at least six independent dimensions (test quality, observability, security, mobile completeness, reliability, code audit) and is too broad for a single spec.

This spec scopes only the **first** of those: a structured read-only audit that produces a prioritized findings list. Findings cluster into 3–5 candidate downstream specs that fix bugs, add features, and close gaps. No code changes during the audit itself.

## 2. Scope

**In scope**
- Backend Go (`backend/`) — primary focus, ~116 files.
- Mobile Flutter (`mobile/`) — secondary; recently audited but worth a deliberate pass on error UX, offline behavior, push completeness.
- Infrastructure (`docker-compose.yml`, `Makefile`, `.github/workflows/`, `migrations/`) — light pass for supply-chain, build-time, migration reversibility.

**Out of scope**
- Home Assistant internals, MQTT broker internals, third-party libraries — only how *we* call them.
- Performance benchmarking under load — separate spec if needed.
- Production / Kubernetes deployment — pet project on Docker Compose.
- Fixes — see §10 (Out of Scope).

**Discipline: read-only.** No code commits during the audit except the findings doc itself. Reason: see the full picture before committing to a fix sequence; some findings reveal others.

## 3. Methodology — three phases

### Phase 1 — Automated scan (cheap, broad)
Run all available linters/scanners. Capture raw output to disk under `raw/`.

Backend (from `backend/`):
- `go vet ./...`
- `staticcheck ./...` (install: `go install honnef.co/go/tools/cmd/staticcheck@latest`)
- `govulncheck ./...` (install: `go install golang.org/x/vuln/cmd/govulncheck@latest`)
- `gosec ./...` (install: `go install github.com/securego/gosec/v2/cmd/gosec@latest`)
- `go test -coverprofile=cover.out ./... && go tool cover -func=cover.out`

Mobile (from `mobile/`):
- `flutter analyze`
- `dart fix --dry-run`
- `flutter pub outdated`
- `flutter test --coverage`

Infra:
- `goose status` from `backend/`.
- Container scan: try `trivy fs .` first, fall back to `docker scout cves` if installed; if neither is available, note "container scan skipped — install trivy or docker scout" and proceed.

### Phase 2 — Subsystem deep-read (focused, deep)
Manual code reading per subsystem, weighted by risk tier (§5).

### Phase 3 — Cross-cutting passes (architectural)
Things tools and per-file reading miss: contract drift, error-handling consistency, observability gaps, idempotency, graceful shutdown, secret management.

### Phase 4 — Synthesize & recommend
Cluster findings → propose 3–5 downstream specs.

## 4. Rubric

Every finding maps to one **category** and one **severity**.

**Categories**
1. **Correctness/Bugs** — logic errors, race conditions, off-by-one, swallowed errors, incorrect SQL.
2. **Security** — authn/authz gaps, input validation, secret handling, JWT/refresh, MQTT/WS auth, CVEs, log PII.
3. **Contracts** — REST DTO drift, WS message versioning, MQTT topics, DB schema vs code, migration reversibility.
4. **Reliability** — timeouts, retries, idempotency, graceful shutdown, DB pool, WS backpressure, reconnect.
5. **Observability** — log levels, structured fields, correlation IDs, metrics, traces, health/readiness.
6. **Tests** — coverage gaps, slow tests, missing handler tests, flaky candidates, untested error paths.
7. **Code quality** — dead code, duplication, complexity, oversized files/functions, leaky abstractions.
8. **Mobile UX** — error messages, offline behavior, push completeness, accessibility, i18n.
9. **Infra/Supply chain** — Dockerfile review, CI scans, base-image freshness, build determinism.

**Severity tiers**

| Tier | Definition | Examples | SLA |
|---|---|---|---|
| **P0 — Critical** | Data loss, auth bypass, RCE, secret leak | Missing authz on admin endpoint; secret in repo; SQL injection | Fix before next merge |
| **P1 — High** | Wrong behavior in normal flow, silent corruption, security weakness without exploit chain | Schedule overlap accepts conflicts; refresh never expires; driver swallows errors | Next sprint |
| **P2 — Medium** | Degraded UX, observability gaps, missing tests on risky code, contract drift without immediate breakage | No metrics on external calls; handler coverage <50%; DTO field added without version bump | Within a milestone |
| **P3 — Low** | Code smell, dead code, doc gaps, minor inconsistencies | Unused imports; outdated comment; magic numbers | Opportunistic |
| **Info** | Worth recording, not actionable on its own | "Service X uses pattern Y; consider standardizing" | Reference |

**Finding format**
```
### F-NNN — <one-line title>
**Category:** Correctness | Security | Contracts | Reliability | Observability | Tests | Quality | MobileUX | Infra
**Severity:** P0 | P1 | P2 | P3 | Info
**Location:** path/to/file.go:42 (or "cross-cutting" if multi-file)
**Evidence:** <code excerpt or specific behavior>
**Why it matters:** <1-3 sentences — what breaks, when, who feels it>
**Suggested direction:** <sketch, not full fix — "add ctx timeout", "switch to UPSERT", "extract helper">
**Effort:** S (<1h) | M (1-4h) | L (>4h, may need its own spec)
**Blast radius:** local | service | cross-service / contract
```

**Stop rules**
- Once we know category, severity, location, and rough fix direction — stop investigating.
- If the fix needs >30 min of design, mark `F-NNN — needs dedicated spec` and move on.
- P3/Info threshold is strict — if it wouldn't change a decision, drop it.

## 5. Subsystem checklist

Time-budget tag in `[brackets]` reflects expected reading depth.

### Tier 1 — Deep read (60–90 min each)

**`internal/auth` + `platform/tokens`** `[deep]`
- Password hashing: bcrypt cost factor; constant-time compare.
- JWT: signing alg pinned (no `none`); aud + iss validated; expiry sane; clock skew.
- Refresh tokens: rotation on use, revocation, replay detection, storage (DB? hashed?).
- Rate limiting: applied to login + register + refresh; per-IP vs per-user.
- Brute-force: account-lockout? Failed-attempt logging without PII.
- Logout/session invalidation actually invalidates refresh tokens.

**`internal/notification`** `[deep]`
- Trigger correctness: temp/humidity/offline thresholds — what numbers, configurable?
- Debouncing/dedup on flapping sensors.
- List paginated; authz scoped to user/classroom.
- FCM delivery: token storage, expiry, batch send, partial failure.
- Audit log integration: every dispatch recorded.

**`internal/schedule`** `[deep]`
- Overlap detection: timezone, DST edges, week-roll, edit-overlap-with-self off-by-one.
- "Current lesson" near minute boundaries.
- Recurring weekly: holiday handling, single-day overrides.
- Concurrent edits: optimistic locking? Cross-classroom moves.

**`internal/scene`** `[deep]`
- Sequential vs parallel execution; failure of step N → abort or continue.
- Idempotency: re-running mid-execution.
- Authz: who can run; cross-classroom invocation.
- Error propagation per recent commit `3618cb5`.
- Long-running: cancellation, ctx propagation, per-step timeouts.

**`internal/devicectl` + drivers** `[deep]`
- `Driver` interface: every command has timeout, error wrap, no panic on transport failure.
- `generic_http`: URL validation (no SSRF to cloud metadata), Content-Type assumed, response parsing.
- `homeassistant`: token refresh, 401 handling, entity_id existence check, domain → command mapping completeness vs README.
- `smartthings`: token rotation, capability mapping, error semantics.
- Idempotency on retry: ON+ON shouldn't double-toggle.
- Concurrency: serialized per device or free-for-all.

**`internal/realtime/ws` + `internal/realtime`** `[deep]`
- Auth on upgrade: JWT validated on connect; reconnect re-validates.
- Authorization: tenant boundary on which messages a teacher sees.
- Backpressure: slow client doesn't block hub goroutine.
- Goroutine leaks on disconnect; ping/pong + read deadline; max message size.
- Schema: any version field; forward-compat per `contracts.md`.

### Tier 2 — Focused read (30–45 min each)

**`internal/classroom`** — authz scoping, soft-delete cascade, pagination/search/sort.
**`internal/device`** — CRUD authz, classroom binding, driver field validation, JSON config schema per driver, offline detection.
**`internal/sensor`** — bulk insert, validation, history index usage, latest-per-device query plan.
**`internal/analytics`** — aggregation correctness, timezone day buckets, p95 latency vs `< 3s` budget, energy formula consistency.
**`internal/hass`** — onboarding state machine error paths, token persistence + refresh, self-test accuracy. **Investigate 55s test runtime** — file as a finding (Tests/Reliability category) regardless of cause; the cause becomes the suggested direction (real HTTP vs `time.Sleep` vs testcontainers).
**MQTT** — broker auth/ACL (currently anonymous?), topic layout per classroom/tenant, reconnect, QoS, retained-message cleanup.

### Tier 3 — Skim (10–15 min each)

- `internal/server` — routing, middleware order, CORS, body limits.
- `platform/httpx` + middleware — recovery, logging, request ID.
- `platform/i18n` — bundle completeness EN/RU/KK, missing-key behavior.
- `platform/validation` — coverage gaps.
- `platform/postgres` — pool config, retry, ping.
- `cmd/server/main.go` — graceful shutdown order, signal handling.
- `migrations/*.sql` — every migration has `-- +goose Down`.
- `auditlog` — what's recorded vs missed.

### Mobile (`mobile/lib`) `[focused, half-day total]`

- `core/api/endpoints/*` — error envelope handling consistent (`{data}/{error}`).
- `core/ws` — reconnect, backoff, message dedup on reconnect.
- `core/storage` — secure storage usage.
- `core/push` — FCM stub completeness audit.
- `core/router` — auth guards, deep links, back-button traps.
- `features/iot_wizard` — OAuth URL extraction, host-file precondition documented in UI.
- `features/notifications` — empty-state, error UX.
- `features/analytics` — chart data flow, offline behavior.
- i18n: every visible string has EN/RU/KK; missing-key fallback.
- Error UX: `friendlyError` consistency.
- Offline banner triggers; queued mutations on reconnect.
- Accessibility: semantics labels, contrast, dynamic font scaling.

### Infra / CI / Supply chain `[skim, ~30 min]`

- Dockerfile(s) — multi-stage, non-root user, pinned base images, no secrets in layers.
- `docker-compose.yml` — healthchecks, dependency order, volume safety, minimal exposed ports.
- `.github/workflows/ci.yml` — scanners, coverage gate, supply-chain (action SHAs vs tags).
- `.env.example` — placeholders only.
- `migrations/` — every file has Down; no edits to merged migrations.

### Cross-cutting passes (after per-subsystem) `[~2h]`

1. **Contract drift** — every API DTO grep'd against mobile usage; WS message types vs mobile parser.
2. **Error handling consistency** — pick 3 random error paths, trace DB → service → handler → response. Swallowed errors? 500s leaking internals?
3. **Secret scan** — `grep -r "sk-\|api_key\|password\s*=\|secret\s*="` in source.
4. **PII in logs** — grep `log.*` in handlers; user data interpolated?
5. **Metrics/traces presence** — count `metrics.` / `tracer.` calls (likely zero — that's a finding).

## 6. Deliverables

### Primary artifact
Path: `docs/superpowers/audits/2026-05-01-deep-audit.md`

```
# Deep Audit — 2026-05-01

## Executive summary
- N findings: P0=x, P1=y, P2=z, P3=w, Info=k
- Top 5 risks (links to F-NNN)
- Recommended next 3 specs (each pulls from a cluster of findings)

## Methodology
(Condensed copy of §3 + §4)

## Findings
### F-001 — <title>
…
### F-002 — <title>
…

## Tool output appendix
(Summary table of which tools ran, exit code, hit count.
Full raw outputs live under raw/, not inlined here.)

## Coverage snapshot
(Per-package coverage table, dated)

## Cross-cutting observations
- Contract drift summary
- Observability gap summary
- Test gap summary
```

### Secondary artifacts
- `docs/superpowers/audits/2026-05-01-deep-audit/raw/` — raw tool outputs (`govulncheck.txt`, `staticcheck.txt`, `gosec.json`, `flutter-analyze.txt`, `pub-outdated.txt`, `coverage.txt`).
- `docs/superpowers/audits/2026-05-01-deep-audit/next-specs.md` — 1-page list of "specs to spawn from these findings", each a 1–3 line stub.

### Numbering
Findings numbered `F-001` upward in discovery order. Severity is a field. Stable IDs let downstream specs reference them.

## 7. Execution plan

| Phase | Estimate | Description |
|---|---|---|
| 1 — Automated scan | ~45 min | Install missing tools, run all linters/scanners, capture to `raw/`, draft any non-noise hit as `F-NNN`. |
| 2 — Subsystem deep-read | 6–8h | Tier 1 → Tier 2 → Tier 3 → Mobile → Infra. Findings logged as discovered, not batched at end. |
| 3 — Cross-cutting passes | ~2h | Contract drift, error-handling traces, secret scan, PII scan, metrics/traces presence. |
| 4 — Synthesize & recommend | ~1h | Sort findings, write executive summary, cluster into `next-specs.md`. |

**Total: 10–12 hours of focused execution.**

## 8. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Findings list bloats with low-value items | P3/Info threshold strict — drop if it wouldn't change a decision |
| Long audit + no fixes feels unproductive | Each finding has clear "next step"; value visible before fixes ship |
| Tooling install fails on host | Fall back gracefully — note "tool X not run" in coverage section |
| Memory/context loss across long session | Findings written to disk as discovered, not held in head |
| Auditor bias toward easy areas | Tier 1 strict; if subsystem looks clean, document the checks performed |
| Audit ages as code evolves | Dated filename; future audits add new dated docs, never edit old ones |

## 9. Done criteria

- All Tier 1 subsystems have a checklist line item with either a finding or a documented "no finding" note.
- All automated tools either ran with output captured, or are explicitly noted "not run because X".
- `next-specs.md` lists at least 3 candidate specs derived from finding clusters.
- Executive summary fits on one screen.

## 10. Out of scope (explicit non-goals)

- Fixing anything found.
- Performance benchmarking under load.
- Full third-party dependency review beyond CVE scanning.
- Production deployment review (no prod env).
- Comparing against a prior audit (this is the first).

## 11. After this spec

Implementation of this audit becomes the next plan via `superpowers:writing-plans`. The audit's output (`next-specs.md`) seeds the next 2–3 brainstorming cycles for actual fixes/features.
