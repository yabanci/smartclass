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
##### notification
##### schedule
##### scene
##### devicectl + drivers
##### realtime/ws

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
