# Deep Audit — 2026-05-01

> Read-only audit (with iterative fixes per user direction). Source spec: `docs/superpowers/specs/2026-05-01-deep-audit-design.md`.
> Plan: `docs/superpowers/plans/2026-05-01-deep-audit-execution.md`.

## Executive summary
_Filled in at end of Phase 4._

## Methodology
Categories: Correctness | Security | Contracts | Reliability | Observability | Tests | Quality | MobileUX | Infra.
Severity: P0 (critical) → P1 (high) → P2 (medium) → P3 (low) → Info.
See spec §3-§4 for full rubric.

## Tool output appendix

| Tool | Ran? | Initial hits | Post-fix | Raw file |
|---|---|---|---|---|
| go vet | yes | 0 | 0 | `raw/go-vet.txt` |
| staticcheck | yes | 3 (S1016) | **0** | `raw/staticcheck.txt` |
| govulncheck | yes | 8 (deps) | **0** | `raw/govulncheck.txt` |
| gosec | yes | 6 (4 HIGH SSRF, 1 MED password marshal, 1 MED file include) | **0** (4 mitigated by validation, 2 annotated `#nosec` with rationale) | `raw/gosec.txt` |
| go coverage | yes | 26.1% total | 26.1% (will improve in later iterations) | `raw/coverage.txt` |
| flutter analyze | yes | 0 | 0 | `raw/flutter-analyze.txt` |
| dart fix --dry-run | yes | "Nothing to fix!" | — | `raw/dart-fix.txt` |
| flutter pub outdated | yes | 23 newer-incompat (transitive flutter SDK lock) | not actionable in this iteration | `raw/pub-outdated.txt` |
| flutter test | yes | 59 passed | 59 passed | `raw/flutter-test.txt` |
| goose status | skipped | — | needs live DB | `raw/goose-status.txt` (not generated) |
| trivy / docker scout | skipped | — | not installed locally | (skipped) |

## Coverage snapshot (Backend)

| Package | Coverage |
|---|---|
| smartclass/cmd/server | 0.0% |
| smartclass/internal/analytics | 15.9% |
| smartclass/internal/auditlog | 0.0% |
| smartclass/internal/auth | 79.5% |
| smartclass/internal/classroom | 13.7% |
| smartclass/internal/classroom/classroomtest | 0.0% |
| smartclass/internal/config | 0.0% |
| smartclass/internal/device | 16.2% |
| smartclass/internal/devicectl | 100.0% |
| smartclass/internal/devicectl/drivers/generic | 73.5% |
| smartclass/internal/devicectl/drivers/homeassistant | 65.3% |
| smartclass/internal/devicectl/drivers/smartthings | 73.7% |
| smartclass/internal/hass | 41.1% |
| smartclass/internal/notification | 26.3% |
| smartclass/internal/platform/hasher | 87.5% |
| smartclass/internal/platform/i18n | 59.3% |
| smartclass/internal/platform/tokens | 83.9% |
| smartclass/internal/platform/validation | 68.4% |
| smartclass/internal/realtime/ws | 43.3% |
| smartclass/internal/scene | 18.8% |
| smartclass/internal/schedule | 17.0% |
| smartclass/internal/sensor | 14.8% |
| smartclass/internal/server | 0.0% |
| smartclass/internal/user | 35.1% |
| **Total** | **26.1%** |

**Below 30% (high risk):** analytics, auditlog (0%), classroom, config (0%), device, notification, scene, schedule, sensor, server (0%), cmd/server (0%).
**Above 70%:** auth, devicectl (interface), drivers/generic, drivers/smartthings, hasher, tokens.

## Findings

### Phase 1 — Automated scan

#### F-001 — 8 transitive CVEs in dependencies
**Category:** Security
**Severity:** P2
**Location:** `backend/go.mod` (pgx/v5, go-chi/chi/v5, x/net, x/crypto)
**Evidence:** `govulncheck -show verbose ./...` reported 8 vulnerabilities (GO-2026-4772, GO-2026-4771, GO-2026-4441, GO-2026-4440, GO-2026-4316, GO-2025-4135, GO-2025-4134, GO-2025-4116). None reachable from our code (`Your code is affected by 0 vulnerabilities`), but all in modules we require.
**Why it matters:** Reachability analysis is good but not perfect; supply-chain hygiene says fix flagged versions even when not reachable.
**Suggested direction:** `go get -u <each module>@latest && go mod tidy`.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** in commit (deps bumped: pgx 5.7.4→5.9.2, chi 5.2.2→5.2.5, x/crypto 0.36→0.50, x/net 0.38→0.53). govulncheck now reports 0.

#### F-002 — staticcheck S1016: 3 struct-conversion opportunities
**Category:** Quality
**Severity:** P3
**Location:** `backend/internal/classroom/handler.go:134`, `backend/internal/device/handler.go:141`, `backend/internal/user/handler.go:60`
**Evidence:** Manual field-by-field copy from `*Request` to `*Input` when struct shapes are identical.
**Why it matters:** Maintenance hazard — adding a field to one struct requires touching the handler too. Type conversion `Input(req)` is shorter and lint-clean.
**Suggested direction:** `s.Update(... UpdateInput(req))`.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** in commit. staticcheck now 0 hits.

#### F-003 — gosec G704 SSRF: HA flowID input not validated
**Category:** Security
**Severity:** P2
**Location:** `backend/internal/hass/client.go` (StepFlow, AbortFlow)
**Evidence:** Caller-supplied `flowID` was concatenated into URL path without validation. An authenticated admin could craft `flowID="../../../some-internal-endpoint"` to redirect the request, or include `?` to break the URL parser.
**Why it matters:** Defense in depth — even though baseURL is operator-configured (so this isn't classic SSRF), traversal could let an admin hit unintended HA endpoints.
**Suggested direction:** Regex-validate flowID; reject anything not matching `^[A-Za-z0-9_-]{1,128}$`.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — added `flowIDPattern` regex + `ErrInvalidFlowID`; both `StepFlow` and `AbortFlow` now validate.

#### F-004 — gosec G117: HA password marshal flagged (false positive)
**Category:** Security
**Severity:** Info
**Location:** `backend/internal/hass/client.go:97-101` (CreateOwner)
**Evidence:** Marshaling `onboardUserReq{... Password: ...}` triggered G117 ("secret pattern").
**Why it matters:** This is the HA onboarding wire format — sending the password to HA is the entire purpose of the call.
**Suggested direction:** Document with `#nosec G117` + rationale comment.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — `#nosec G117` annotation added with rationale.

#### F-005 — gosec G304: i18n LoadDir flagged (false positive)
**Category:** Security
**Severity:** Info
**Location:** `backend/internal/platform/i18n/i18n.go:78`
**Evidence:** `os.ReadFile(filepath.Join(dir, e.Name()))` triggered G304 ("file inclusion via variable").
**Why it matters:** `e.Name()` comes from `os.ReadDir` (returns basenames, not paths) and is filtered by `.json` suffix and the `supported` language map. No untrusted input reaches the read.
**Suggested direction:** Wrap with `filepath.Base(e.Name())` belt-and-braces; annotate `#nosec G304`.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — `filepath.Base` added + `#nosec G304` annotation with rationale.

#### F-006 — Backend total test coverage 26.1%
**Category:** Tests
**Severity:** P2
**Location:** cross-cutting (handler/service layers across most packages)
**Evidence:** `go test -cover ./...` total 26.1%. Service-layer packages 13–26%; HTTP handler packages effectively untested; `cmd/server`, `auditlog`, `config`, `internal/server` at 0%.
**Why it matters:** Refactoring these subsystems carries higher risk than necessary. Most bugs that escape to production live in untested handler paths (ctx usage, status codes, validation errors).
**Suggested direction:** **Needs dedicated spec.** Target: 60% on every Tier 1+2 package (auth, notification, schedule, scene, classroom, device, sensor, analytics, hass) by adding `_test.go` for handlers + service edge cases. Use `httptest.NewRecorder` + chi router for handler tests.
**Effort:** L
**Blast radius:** service

#### F-007 — Mobile: 23 packages constrained to older versions (Flutter SDK lock)
**Category:** Infra
**Severity:** Info
**Location:** `mobile/pubspec.lock`
**Evidence:** `flutter pub outdated` reports 23 transitive packages locked older than resolvable. Mostly `_macros`/`_macros_internal`/`shelf`/etc. — all transitive, locked by Flutter SDK 3.41.7's pubspec constraints.
**Why it matters:** Not actionable without a Flutter SDK upgrade. Track for future SDK bump.
**Suggested direction:** Re-evaluate after Flutter SDK upgrade (CI is pinned to 3.41.7).
**Effort:** S
**Blast radius:** local

#### F-008 — Mobile: package `js` is discontinued (transitive)
**Category:** Infra
**Severity:** P3
**Location:** `mobile/pubspec.lock` (transitive via test/web tooling)
**Evidence:** `flutter pub outdated` warns: "Package js has been discontinued."
**Why it matters:** Transitive only; will be removed by upstream eventually.
**Suggested direction:** Track; no action needed unless the SDK bump fails because of it.
**Effort:** S
**Blast radius:** local

### Phase 2 — Subsystem deep-read

#### Tier 1
##### auth + tokens

#### F-009 — Refresh tokens have no rotation, revocation, or replay detection
**Category:** Security
**Severity:** P1
**Location:** `backend/internal/auth/service.go` (Refresh), `backend/internal/platform/tokens/tokens.go`
**Evidence:** Original `Service.Refresh` parsed the refresh JWT and issued a fresh pair without invalidating the old one. The old refresh token remained valid until expiry, and there was no `/auth/logout` to revoke. Refresh-token state was not stored in DB at all.
**Why it matters:** A stolen refresh token works until expiry — typically 7+ days. No way for the user to invalidate it. No way to detect a replay attack (legitimate user logs in again, attacker still holds the original token). Standard OWASP refresh-token guidance is: rotate on use + persist jti + revoke on logout.
**Suggested direction:** New `refresh_tokens(jti, user_id, expires_at, used_at, revoked_at)` table. `RefreshStore` interface (postgres + in-memory impl for tests). Service.Refresh: status check → mark used → revoke-all-on-replay-detection → issue new pair (which inserts new jti). Add `Service.Logout` and `POST /api/v1/auth/logout`.
**Effort:** L
**Blast radius:** service (auth, no contract change for clients besides new endpoint)
**Status:** **FIXED** — migration `00013_refresh_tokens.sql`, `auth.RefreshStore` interface, `auth.PostgresRefreshStore`, `auth.MemRefreshStore` (for tests), `Service.Refresh` rotated/replay-checked, new `Service.Logout` + `POST /auth/logout`. 5 new tests cover rotation, replay detection (revokes all sessions), logout, deleted-user rejection, garbage rejection.

#### F-010 — JWT Parse did not validate `iss`, lacked clock-skew leeway, did not pin signing alg
**Category:** Security
**Severity:** P2
**Location:** `backend/internal/platform/tokens/tokens.go:109` (Parse)
**Evidence:** Issuer was set when signing but never verified on parse. No `WithLeeway` — strictly past `exp` was rejected (real-world clock drift between containers can cause spurious 401s). `WithValidMethods` was not set, so the alg confusion attack surface was wider than necessary (manual check existed but defense in depth missing).
**Why it matters:** Different smartclass deployments could conceivably share a JWT signing secret while having different issuers; without `iss` validation, a token from environment A would authenticate against B. Clock-skew leeway prevents flaky 401s.
**Suggested direction:** `WithIssuer(j.issuer)` + `WithLeeway(30*time.Second)` + `WithValidMethods([]string{"HS256"})` + `WithExpirationRequired()` in ParseWithClaims options.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — all four options added; new tests `TestJWT_RejectsWrongIssuer` and `TestJWT_AllowsClockSkewWithinLeeway` cover the change.

#### F-011 — `/healthz` and `/readyz` returned 200 unconditionally
**Category:** Observability
**Severity:** P2
**Location:** `backend/internal/server/server.go:67-68,125`
**Evidence:** Both routes pointed at `healthz` which returned 200 with no dependency check. A deployment where Postgres was unreachable would still report Ready=true and receive traffic.
**Why it matters:** Per `pet/CLAUDE.md` §13 ("Readiness fails if: DB unreachable…"), `/readyz` must reflect real dependency state. Otherwise Kubernetes/load balancer never sheds traffic to a broken instance.
**Suggested direction:** Define `ReadinessChecker` interface; have `*postgres.DB` implement `Ready(ctx)` via Pool.Ping with 2s timeout; wire `Deps.Readiness` from main.go.
**Effort:** S
**Blast radius:** service
**Status:** **FIXED** — `server.ReadinessChecker` interface, `postgres.DB.Ready`, `/readyz` now returns 503 with reason on DB ping failure. `/healthz` (liveness) stays decoupled.

#### F-012 — No request-body size limit
**Category:** Reliability / Security
**Severity:** P2
**Location:** `backend/internal/server/server.go` (middleware stack)
**Evidence:** No `http.MaxBytesReader` middleware in the chain. A client could POST a 1 GB body to any endpoint and tie up server memory parsing it.
**Why it matters:** DoS amplification on JSON-decoding endpoints. Also catches misbehaving clients early with a clean 413 instead of timing out.
**Suggested direction:** New `middleware.BodyLimit(1 MiB)` middleware applied globally.
**Effort:** S
**Blast radius:** service
**Status:** **FIXED** — `BodyLimit` middleware + 1 MiB cap + 2 unit tests.

#### F-013 — No request-ID middleware; logs lack correlation IDs
**Category:** Observability
**Severity:** P3
**Location:** `backend/internal/platform/httpx/middleware/logger.go`, `server.go`
**Evidence:** Request log line included method/path/status/bytes/duration but no correlation ID. Tracing a single request across logs (or correlating to a client bug report) was impossible without `grep`-by-timestamp.
**Why it matters:** Per `pet/observability.md` rule "Include a correlation ID in every log line on the request path."
**Suggested direction:** New `middleware.RequestID` that generates UUID (or trusts inbound `X-Request-Id` ≤128 chars), echoes back in response header, adds to context. `RequestLogger` reads it and adds `request_id` zap field.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — `middleware.RequestID` + 3 unit tests; `RequestLogger` includes `request_id` field when present.

#### F-014 — Auth tokens accepted via `?access_token=` query param
**Category:** Security / Observability
**Severity:** P3
**Location:** `backend/internal/platform/httpx/middleware/auth.go:64`
**Evidence:** WebSocket upgrades fall back to a query param because browsers can't set Authorization headers on WS handshakes. Tokens in query strings get logged by reverse proxies, browser history, etc.
**Why it matters:** Documented anti-pattern. The mobile client uses WS so the fallback is currently load-bearing, but it widens the leakage surface.
**Suggested direction:** **Needs dedicated spec.** Replace with a short-lived (60 s) WS ticket: client calls `POST /ws/ticket` with bearer, server returns one-time ticket, client uses `?ticket=...` (single-use, server-side rotation). Simpler than per-request JWT, no PII in proxy logs.
**Effort:** M
**Blast radius:** service (mobile WS client + server change)
##### notification

#### F-015 — Notification Engine cooldown map grew unbounded
**Category:** Reliability
**Severity:** P3
**Location:** `backend/internal/notification/trigger.go:56` (throttle)
**Evidence:** `lastAlert` map keyed on `classroomID:deviceID:rule` was inserted into on every alert and never cleaned up. Devices that are decommissioned still occupied a map entry forever.
**Why it matters:** Slow memory leak. Pet-project scale never sees it; not so for a long-running deployment with high device churn.
**Suggested direction:** Lazy GC pass when map size crosses a high-water mark; delete entries older than the cooldown window.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — high-water-mark eviction in `throttle()`.

#### F-016 — Notification thresholds are global, not per-classroom
**Category:** Reliability
**Severity:** P3 (Info if a future feature)
**Location:** `backend/internal/notification/trigger.go:27-34` (Rules)
**Evidence:** `Rules{TemperatureHighC, TemperatureLowC, HumidityHigh}` is a single struct constructed at startup and reused across every classroom.
**Why it matters:** A chemistry lab and a kindergarten have different "high temperature" definitions. With global thresholds, getting one right means getting the other wrong.
**Suggested direction:** **Needs dedicated spec.** Per-classroom thresholds stored in DB with sensible defaults; admin UI to override.
**Effort:** M
**Blast radius:** service (UI + DB + service)

##### schedule

#### F-017 — Schedule "current lesson" uses server-local time, not classroom timezone
**Category:** Correctness
**Severity:** P3 (single-tenant pet project; would be P1 multi-region)
**Location:** `backend/internal/schedule/service.go:170` (Current)
**Evidence:** `mins := TimeOfDay(now.Hour()*60 + now.Minute())` reads the server's wall-clock time. The lesson `StartsAt`/`EndsAt` are stored as `TimeOfDay` minutes-since-midnight, presumed to be in the school's timezone.
**Why it matters:** Single-school single-server is fine (server's TZ is the school's TZ). Multi-region deployment would silently match the wrong lesson.
**Suggested direction:** Add `timezone` to classrooms (default "Asia/Almaty"); convert `time.Now()` to that TZ before extracting minutes.
**Effort:** M
**Blast radius:** service (DB column + migration + frontend display)

##### scene

#### F-018 — Scene execution had no per-step timeout
**Category:** Reliability
**Severity:** P2
**Location:** `backend/internal/scene/service.go` (Run)
**Evidence:** Each step called `s.devices.Execute(ctx, ...)` with the request context. The driver client has its own Timeout (5–15s), but the call path includes auth/repo lookups upstream. A single slow step would block the entire scene goroutine indefinitely if the inner Timeout was zero or absent for any reason.
**Why it matters:** A 10-step scene with one stuck device blocks the whole batch. Defense in depth on top of driver timeouts.
**Suggested direction:** Wrap each step in `context.WithTimeout(ctx, 10*time.Second)`. Honour caller cancellation between steps.
**Effort:** S
**Blast radius:** local
**Status:** **FIXED** — per-step `context.WithTimeout(10s)`; loop also breaks on `ctx.Err()`.

##### devicectl + drivers

#### F-019 — Driver layer review: timeouts present, errors wrapped, panics avoided
**Category:** Quality
**Severity:** Info
**Location:** `backend/internal/devicectl/drivers/{generic,homeassistant,smartthings}/`
**Evidence:** Each driver constructs an `http.Client` with explicit timeout (5s/10s/15s). Errors wrapped with `%w`. No `panic()` on transport errors.
**Why it matters:** Confidence point — the driver layer holds up. SSRF concern in `generic_http` (no rejection of internal addresses like `169.254.169.254` for cloud metadata) is real but admins set the URL, not end users → not in this audit's threat model.
**Suggested direction:** None for now. A future hardening spec could deny RFC1918+link-local in `generic_http` config validation.
**Status:** **NO IMMEDIATE FIX** — documented as Info.

##### realtime/ws

#### F-020 — WebSocket subscription bypassed tenant boundaries
**Category:** Security
**Severity:** P1
**Location:** `backend/internal/realtime/ws/handler.go:67` (parseTopics)
**Evidence:** `parseTopics` accepted any string from `?topic=` query and registered it as a subscription. A teacher could pass `?topic=classroom:<other-uuid>:devices` and silently observe events from a classroom they had no membership in.
**Why it matters:** Broken access control — a multi-tenant CCTV-adjacent product cannot let one teacher see another classroom's realtime device/sensor stream.
**Suggested direction:** Strict allowlist: `user:<self>:notifications` (auto-added) and `classroom:<id>:*` only when `classroom.Service.Authorize(p, id, false)` succeeds. Reject any other shape with 403.
**Effort:** M
**Blast radius:** service (WS handler + main.go wiring)
**Status:** **FIXED** — `TopicAuthorizer` interface, strict topic parser, `Handler.authorizeTopics` rejects foreign-user, foreign-classroom, and unknown-shape topics; 6 new unit tests.

#### F-021 — WebSocket CheckOrigin allows any origin
**Category:** Security
**Severity:** P3
**Location:** `backend/internal/realtime/ws/handler.go:40` (upgrader.CheckOrigin)
**Evidence:** `CheckOrigin: func(_ *http.Request) bool { return true }`.
**Why it matters:** With Bearer-JWT-in-query authentication, CSRF-equivalent for WS is partially mitigated (no cookie-based session to hijack). Still defense-in-depth gap; a malicious page that can read tokens (XSS) can also open a WS connection more easily.
**Suggested direction:** Reuse the CORS allowed-origins list. Track in the WS-ticket spec (F-014).
**Effort:** S
**Blast radius:** local

#### F-022 — WebSocket message schema has no version field
**Category:** Contracts
**Severity:** P3
**Location:** `backend/internal/realtime/event.go` (Event struct)
**Evidence:** `realtime.Event{Topic, Type, Payload}` — no `version` per `pet/contracts.md` rule "Every message carries a `version` field".
**Why it matters:** Future schema changes can't be rolled out incrementally if consumers can't tell which version they're parsing.
**Suggested direction:** Add `Version int` (default 1); mobile parser tolerates unknown fields already.
**Effort:** S
**Blast radius:** cross-service (mobile + backend in lock-step)
**Note:** marked `needs dedicated spec` — touches a contract.

#### Tier 2
##### classroom
##### device
##### sensor
##### analytics
##### hass
##### MQTT

#### Tier 3
##### server / httpx / middleware
##### platform: i18n / validation / postgres / main / auditlog / migrations

#### Mobile
##### core
##### features
##### UX / i18n / accessibility / offline

#### Infra / CI / Supply chain

### Phase 3 — Cross-cutting
##### Contract drift
##### Error handling consistency
##### Secret scan
##### PII in logs
##### Metrics/traces presence

## Cross-cutting observations
_Filled by Phase 4._

## Fixes applied during audit
_Each fix gets a one-line entry: `F-NNN — fixed in <commit>`._
