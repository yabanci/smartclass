# Deep Audit Execution — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute the read-only audit defined in `docs/superpowers/specs/2026-05-01-deep-audit-design.md`, producing a prioritized findings doc plus raw tool outputs and `next-specs.md`.

**Architecture:** Read-only investigation. Findings written to disk as discovered, never batched at end (avoids context loss). One finding = one numbered entry (`F-001` upward) using the spec's rubric. Phases run sequentially: automated scan → subsystem deep-read → cross-cutting passes → synthesize. **No source-code changes** during execution; only the audit/raw/next-specs files are committed.

**Tech Stack:** Go 1.25, Flutter 3.41.7, Postgres 16, MQTT, Home Assistant. Audit tools: `go vet`, `staticcheck`, `govulncheck`, `gosec`, `flutter analyze`, `dart fix --dry-run`, `flutter pub outdated`, `goose`, `trivy`/`docker scout`.

**Output paths:**
- `docs/superpowers/audits/2026-05-01-deep-audit.md` — primary findings doc
- `docs/superpowers/audits/2026-05-01-deep-audit/raw/` — raw tool outputs
- `docs/superpowers/audits/2026-05-01-deep-audit/next-specs.md` — downstream spec stubs

**Finding format (used by every task that adds a finding):**
```
### F-NNN — <one-line title>
**Category:** Correctness | Security | Contracts | Reliability | Observability | Tests | Quality | MobileUX | Infra
**Severity:** P0 | P1 | P2 | P3 | Info
**Location:** path/to/file.go:42 (or "cross-cutting")
**Evidence:** <code excerpt or specific behavior>
**Why it matters:** <1-3 sentences>
**Suggested direction:** <sketch — "add ctx timeout", "switch to UPSERT">
**Effort:** S | M | L
**Blast radius:** local | service | cross-service
```

**Stop rules per finding:**
- Once category, severity, location, fix direction known — stop investigating.
- If fix needs >30 min of design → mark `needs dedicated spec`, move on.
- P3/Info threshold strict: drop if it wouldn't change a decision.

**Per-task discipline:**
- After each subsystem task, append findings to the audit doc OR write a "no findings" line under that subsystem heading.
- Increment finding numbers monotonically across tasks. Never renumber.
- Commit at task end.

---

## Phase 0 — Bootstrap

### Task 0: Create audit directory structure and skeleton

**Files:**
- Create: `docs/superpowers/audits/2026-05-01-deep-audit.md`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/.gitkeep`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/next-specs.md`

- [ ] **Step 1: Create directories**

```bash
mkdir -p docs/superpowers/audits/2026-05-01-deep-audit/raw
touch docs/superpowers/audits/2026-05-01-deep-audit/raw/.gitkeep
```

- [ ] **Step 2: Write audit doc skeleton**

Create `docs/superpowers/audits/2026-05-01-deep-audit.md` with this content:

```markdown
# Deep Audit — 2026-05-01

> Read-only audit. Source spec: `docs/superpowers/specs/2026-05-01-deep-audit-design.md`.
> Plan: `docs/superpowers/plans/2026-05-01-deep-audit-execution.md`.

## Executive summary
_Filled in at end of Phase 4._

## Methodology
Categories: Correctness | Security | Contracts | Reliability | Observability | Tests | Quality | MobileUX | Infra.
Severity: P0 (critical) → P1 (high) → P2 (medium) → P3 (low) → Info.
See spec §3-§4 for full rubric.

## Tool output appendix
| Tool | Ran? | Exit | Hit count | Raw file |
|---|---|---|---|---|
_Filled by Task 5._

## Coverage snapshot
_Filled by Task 5 (backend) and Task 3 (mobile)._

## Findings

### Phase 1 — Automated scan
_Findings F-001..F-NNN from automated tools._

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
```

- [ ] **Step 3: Write next-specs.md skeleton**

Create `docs/superpowers/audits/2026-05-01-deep-audit/next-specs.md` with:

```markdown
# Downstream specs — derived from 2026-05-01 deep audit

_Populated in Phase 4 by clustering findings._

## Candidate spec 1: TBD
## Candidate spec 2: TBD
## Candidate spec 3: TBD
```

- [ ] **Step 4: Commit skeleton**

```bash
git add docs/superpowers/audits/
git -c commit.gpgsign=false commit -m "docs(audit): scaffold 2026-05-01 deep-audit skeleton"
```

---

## Phase 1 — Automated scan

### Task 1: Install Go scanning tools

**Files:** No source changes; installs go binaries to `$GOPATH/bin`.

- [ ] **Step 1: Check what's already installed**

```bash
which staticcheck govulncheck gosec
```

- [ ] **Step 2: Install missing tools**

For each tool not present, run from any directory:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

- [ ] **Step 3: Verify install**

```bash
staticcheck -version
govulncheck -version
gosec -version
```

Expected: each prints a version string. If any fails, note in Task 5 as "tool X not installed — skipped" and proceed.

- [ ] **Step 4: No commit** (tool install is environment-only, nothing to commit).

---

### Task 2: Run Go scanners and capture output

**Files:**
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/go-vet.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/staticcheck.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/govulncheck.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/gosec.json`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/coverage.txt`

- [ ] **Step 1: Run go vet from backend/**

```bash
(cd backend && go vet ./... ) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/go-vet.txt
```

Expected: 0 lines if clean; stderr lines if hits. Don't fail the task on hits — that's the point.

- [ ] **Step 2: Run staticcheck**

```bash
(cd backend && staticcheck ./... ) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/staticcheck.txt || true
```

The `|| true` keeps the pipe alive even if staticcheck exits non-zero (which it does on hits).

- [ ] **Step 3: Run govulncheck**

```bash
(cd backend && govulncheck ./... ) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/govulncheck.txt || true
```

- [ ] **Step 4: Run gosec (JSON output for stable parsing)**

```bash
(cd backend && gosec -fmt=json -out=/tmp/gosec.json -quiet ./... ) || true
mv /tmp/gosec.json docs/superpowers/audits/2026-05-01-deep-audit/raw/gosec.json
```

If gosec wasn't installed, write a stub:

```bash
# only if gosec missing
echo '{"Issues":[],"Stats":{"files":0,"lines":0,"nosec":0,"found":0},"_skipped":"gosec not installed"}' > docs/superpowers/audits/2026-05-01-deep-audit/raw/gosec.json
```

- [ ] **Step 5: Run coverage**

```bash
(cd backend && go test -coverprofile=cover.out ./... && go tool cover -func=cover.out) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/coverage.txt
rm -f backend/cover.out
```

- [ ] **Step 6: Commit raw outputs**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit/raw/
git -c commit.gpgsign=false commit -m "audit(phase1): capture go vet/staticcheck/govulncheck/gosec/coverage"
```

---

### Task 3: Run Flutter analyzers and capture output

**Files:**
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/flutter-analyze.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/dart-fix.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/pub-outdated.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/flutter-test.txt`

- [ ] **Step 1: Ensure Flutter deps**

```bash
(cd mobile && flutter pub get) 2>&1 | tail -5
```

Expected: "Got dependencies!" or similar success line.

- [ ] **Step 2: Run flutter analyze**

```bash
(cd mobile && flutter analyze) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/flutter-analyze.txt || true
```

- [ ] **Step 3: Run dart fix dry-run**

```bash
(cd mobile && dart fix --dry-run) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/dart-fix.txt || true
```

- [ ] **Step 4: Run pub outdated**

```bash
(cd mobile && flutter pub outdated --no-color) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/pub-outdated.txt || true
```

- [ ] **Step 5: Run flutter test (without --coverage to avoid lcov dep)**

```bash
(cd mobile && flutter test --reporter=expanded) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/flutter-test.txt || true
```

Expected: tail line says "All tests passed!" If failures, note in Task 5.

- [ ] **Step 6: Commit raw outputs**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit/raw/
git -c commit.gpgsign=false commit -m "audit(phase1): capture flutter analyze/dart-fix/pub-outdated/flutter-test"
```

---

### Task 4: Run infra scanners

**Files:**
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/goose-status.txt`
- Create: `docs/superpowers/audits/2026-05-01-deep-audit/raw/container-scan.txt`

- [ ] **Step 1: Check goose presence and status**

If goose binary is available locally:

```bash
which goose && (cd backend && goose -dir migrations postgres "$DATABASE_URL" status) 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/goose-status.txt || echo "goose not installed or DATABASE_URL unset — skipped" > docs/superpowers/audits/2026-05-01-deep-audit/raw/goose-status.txt
```

Don't spin up a DB just for this — `goose status` against a live DB is best-effort.

- [ ] **Step 2: Container vuln scan**

Try trivy first, then docker scout:

```bash
if command -v trivy >/dev/null 2>&1; then
  trivy fs --scanners vuln --severity HIGH,CRITICAL backend/ 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/container-scan.txt || true
elif command -v docker >/dev/null 2>&1 && docker scout --help >/dev/null 2>&1; then
  docker scout cves --only-severity high,critical fs://./backend 2>&1 | tee docs/superpowers/audits/2026-05-01-deep-audit/raw/container-scan.txt || true
else
  echo "container scan skipped — install trivy (brew install trivy) or docker scout" > docs/superpowers/audits/2026-05-01-deep-audit/raw/container-scan.txt
fi
```

- [ ] **Step 3: Commit raw outputs**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit/raw/
git -c commit.gpgsign=false commit -m "audit(phase1): capture goose/container-scan outputs"
```

---

### Task 5: Triage automated output → draft findings

**Files:**
- Modify: `docs/superpowers/audits/2026-05-01-deep-audit.md` (fill in Tool output appendix, Coverage snapshot, append findings under "Phase 1 — Automated scan")

- [ ] **Step 1: Read each raw file and classify**

For each file in `raw/`, read fully, then for each issue decide: actionable finding or noise.

Filter rules:
- **govulncheck**: every imported-and-vulnerable package = a finding. Severity = P1 if production code path, P2 if test-only.
- **staticcheck**: SA-* checks (correctness) = at least P3; ST-* checks (style) = drop unless cluster.
- **gosec**: every `HIGH`/`MEDIUM` = finding (severity P1/P2). Drop `LOW` confidence-LOW combos.
- **go vet**: every line = finding (P2 minimum since vet hits are usually real).
- **flutter analyze**: errors = P1, warnings = P2, infos = drop unless cluster of >5.
- **pub outdated**: only flag direct deps with major version behind (P3); transitive — drop.
- **coverage**: any package <30% line coverage = P2 finding (Tests category). Aggregate across packages, not per-package.

- [ ] **Step 2: Fill Tool output appendix table**

Edit `docs/superpowers/audits/2026-05-01-deep-audit.md`, replace the "_Filled by Task 5._" line under "Tool output appendix" with:

```markdown
| Tool | Ran? | Exit | Hit count | Raw file |
|---|---|---|---|---|
| go vet | yes/no | 0/1 | N | raw/go-vet.txt |
| staticcheck | yes/no | 0/1 | N | raw/staticcheck.txt |
| govulncheck | yes/no | 0/1 | N | raw/govulncheck.txt |
| gosec | yes/no | - | N | raw/gosec.json |
| go coverage | yes | 0 | - | raw/coverage.txt |
| flutter analyze | yes/no | 0/1 | N | raw/flutter-analyze.txt |
| dart fix | yes/no | 0/1 | N | raw/dart-fix.txt |
| flutter pub outdated | yes/no | 0/1 | N | raw/pub-outdated.txt |
| flutter test | yes/no | 0/1 | N | raw/flutter-test.txt |
| goose status | yes/no | - | - | raw/goose-status.txt |
| container scan | yes/no | - | N | raw/container-scan.txt |
```

(Replace `N` with actual hit counts. Replace yes/no based on whether tool ran.)

- [ ] **Step 3: Fill Coverage snapshot**

Replace "_Filled by Task 5..._" under "Coverage snapshot" with a table parsed from `raw/coverage.txt`. Format:

```markdown
| Package | Coverage |
|---|---|
| smartclass/internal/auth | 79.5% |
| smartclass/internal/devicectl | 100.0% |
| ... (every line from `go tool cover -func` total per package) ... |
| **Total (aggregated)** | XX.X% |
```

Add 1-line summary: "Below 30%: list package names. Above 70%: list package names."

- [ ] **Step 4: Append findings**

Under "### Phase 1 — Automated scan", append each draft finding using the standard format. Number monotonically starting at F-001. **Inline example only — replace with real findings:**

```markdown
### F-001 — govulncheck: <CVE-ID> in <package>@<version>
**Category:** Security
**Severity:** P1
**Location:** backend/go.sum (transitive: <package>)
**Evidence:** govulncheck reports <CVE-ID> reachable from <our-call-chain> (raw/govulncheck.txt)
**Why it matters:** <one-line CVE impact>
**Suggested direction:** Bump <package> to >=<fixed-version>; run `go mod tidy`.
**Effort:** S
**Blast radius:** local
```

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(phase1): triage automated output into findings"
```

---

## Phase 2 — Subsystem deep-read

**Per-task pattern (applies to Tasks 6–21):**
1. Read every file listed in "Files to read".
2. Run grep checks listed.
3. Walk the checklist from spec §5 against what you read.
4. For each violation: append a finding under the subsystem's heading. For each checklist item with no violation: write `- [x] <check>: OK` under the subsystem heading.
5. Commit.

If a finding's fix needs >30 min of design, append `**Note: needs dedicated spec.**` to the finding and stop investigating.

---

### Task 6: Tier 1 — auth + platform/tokens

**Files to read:**
- `backend/internal/auth/service.go`
- `backend/internal/auth/handler.go`
- `backend/internal/auth/dto.go`
- `backend/internal/auth/service_test.go`
- `backend/internal/auth/handler_test.go`
- `backend/internal/platform/tokens/*.go`
- `backend/internal/platform/hasher/*.go`
- `backend/internal/server/*.go` (route registration + middleware order for auth routes)
- `backend/migrations/00001_init.sql` (users + auth schema)
- `backend/migrations/00011_hass_refresh_token.sql` (refresh-token storage hint)

**Append findings under:** `## Findings → Phase 2 → Tier 1 → auth + tokens`

- [ ] **Step 1: Read all files; note bcrypt cost, JWT alg, refresh-token shape**

Open each file in order. Capture:
- bcrypt cost factor (look for `bcrypt.GenerateFromPassword` second arg — should be ≥12).
- JWT signing alg used in `tokens` package (must NOT be `jwt.SigningMethodNone`).
- Whether refresh tokens are rotated on use, hashed at rest, expire.
- Constant-time compare on auth (`subtle.ConstantTimeCompare` or `bcrypt.CompareHashAndPassword` is fine).

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "SigningMethod" backend/internal/platform/tokens/
grep -rn "bcrypt" backend/internal/platform/hasher/ backend/internal/auth/
grep -rn "RateLim\|rate_lim\|rateLim" backend/internal/server/ backend/internal/platform/httpx/
grep -rn "refresh" backend/internal/auth/ backend/internal/platform/tokens/
grep -rn "ConstantTimeCompare\|subtle\." backend/internal/auth/ backend/internal/platform/
```

- [ ] **Step 3: Walk the spec §5 checklist**

For each line below, write either a finding or `- [x] <check>: OK`:
- Password hashing: bcrypt cost factor; constant-time compare.
- JWT: signing alg pinned; aud + iss validated; expiry sane; clock skew.
- Refresh tokens: rotation, revocation, replay detection, storage (hashed?).
- Rate limiting: applied to login + register + refresh.
- Brute-force: account-lockout? Failed-attempt logging without PII.
- Logout/session invalidation actually invalidates refresh tokens.

- [ ] **Step 4: Append to audit doc and commit**

Append findings under the auth heading, then:

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t1): auth + tokens findings"
```

---

### Task 7: Tier 1 — notification

**Files to read:**
- `backend/internal/notification/notification.go`
- `backend/internal/notification/dto.go`
- `backend/internal/notification/service.go`
- `backend/internal/notification/handler.go`
- `backend/internal/notification/postgres.go`
- `backend/internal/notification/repository.go`
- `backend/internal/notification/trigger.go`
- `backend/internal/notification/trigger_test.go`
- `backend/migrations/00007_notifications.sql`
- `backend/migrations/00012_fcm_token.sql`
- `backend/internal/notification/notificationtest/*`

**Append findings under:** `## Findings → Phase 2 → Tier 1 → notification`

- [ ] **Step 1: Read all files; note thresholds, dispatch path, FCM integration**

Capture:
- Hard-coded threshold values (high/low temp, humidity, offline timeout) — should be configurable.
- Dedup/debounce: is there a "last alerted at" check before re-firing?
- FCM: how are tokens stored, are dead tokens purged, partial failure handling.
- Pagination on list endpoint (look for `limit`/`offset` in handler + DTO).
- Authz scoping: does `service.List(...)` filter by user's classroom membership?

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "fcm\|FCM\|firebase" backend/internal/notification/
grep -rn "trigger\|threshold\|dedup\|debounce" backend/internal/notification/
grep -rn "limit\|offset\|page" backend/internal/notification/handler.go backend/internal/notification/dto.go
grep -rn "auditlog\." backend/internal/notification/
```

- [ ] **Step 3: Walk the spec §5 checklist**

For each line, write a finding or `OK`:
- Trigger correctness: temp/humidity/offline thresholds — configurable?
- Debouncing/dedup on flapping sensors.
- List paginated; authz scoped to user/classroom.
- FCM delivery: token storage, expiry, batch send, partial failure.
- Audit log integration: every dispatch recorded.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t1): notification findings"
```

---

### Task 8: Tier 1 — schedule

**Files to read:**
- `backend/internal/schedule/lesson.go`
- `backend/internal/schedule/dto.go`
- `backend/internal/schedule/service.go`
- `backend/internal/schedule/service_test.go`
- `backend/internal/schedule/handler.go`
- `backend/internal/schedule/postgres.go`
- `backend/internal/schedule/repository.go`
- `backend/internal/schedule/scheduletest/*`
- `backend/migrations/00004_schedule.sql`

**Append findings under:** `## Findings → Phase 2 → Tier 1 → schedule`

- [ ] **Step 1: Read all files; note timezone, overlap algorithm, current-lesson query**

Capture:
- Time zone: are times stored UTC? local? tz-naive? Compare DB schema (`TIMESTAMPTZ` vs `TIMESTAMP`) and Go marshaling (`time.Time` parsing).
- Overlap detection: pseudocode of `service.checkOverlap` or equivalent. Edge: `(start, end)` half-open vs closed; same-second adjacency.
- "Current lesson": query that returns the active lesson — does it use `WHERE NOW() BETWEEN start AND end` (closed) or strict `<`/`<=`?
- Recurring weekly logic: holiday flag, single-day overrides — present or absent?
- Concurrent edits: is there any `version` column or `SELECT ... FOR UPDATE`?

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "time\.Now\|tz\|UTC\|Local" backend/internal/schedule/
grep -rn "overlap\|conflict\|between\|BETWEEN" backend/internal/schedule/ backend/migrations/00004_schedule.sql
grep -rn "current\|CurrentLesson\|active" backend/internal/schedule/
```

- [ ] **Step 3: Walk the spec §5 checklist**

- Overlap detection: timezone, DST, week-roll, edit-overlap-with-self off-by-one.
- "Current lesson" near minute boundaries (open vs closed interval).
- Recurring weekly: holiday handling, single-day overrides.
- Concurrent edits: optimistic locking? Cross-classroom moves.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t1): schedule findings"
```

---

### Task 9: Tier 1 — scene

**Files to read:**
- `backend/internal/scene/scene.go`
- `backend/internal/scene/dto.go`
- `backend/internal/scene/service.go`
- `backend/internal/scene/service_test.go`
- `backend/internal/scene/handler.go`
- `backend/internal/scene/postgres.go`
- `backend/internal/scene/repository.go`
- `backend/internal/scene/scenetest/*`
- `backend/migrations/00005_scenes.sql`

Reference recent commit: `git show 3618cb5` for "scene.run() error propagation" context.

**Append findings under:** `## Findings → Phase 2 → Tier 1 → scene`

- [ ] **Step 1: Read all files; note execution model**

Capture:
- Sequential vs parallel: does `Run` use a `for` loop or `errgroup`/`goroutines`?
- Failure mode: on step 3 of 5 failing — does it return early or continue? Is partial success reported?
- Cancellation: does `Run` accept and respect `ctx`?
- Per-step timeout: is each device command wrapped in `context.WithTimeout`?
- Authz: does `Run` check the caller's classroom membership against the scene's classroom?
- Idempotency: if `Run` is called twice concurrently for same scene, what happens?

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "errgroup\|go func\|sync\.WaitGroup" backend/internal/scene/
grep -rn "ctx\b\|context\." backend/internal/scene/service.go
grep -rn "WithTimeout\|Deadline" backend/internal/scene/ backend/internal/devicectl/
git show 3618cb5 -- backend/internal/scene/ 2>/dev/null | head -60
```

- [ ] **Step 3: Walk the spec §5 checklist**

- Sequential vs parallel; failure of step N → abort or continue.
- Idempotency: re-running mid-execution.
- Authz: who can run; cross-classroom invocation.
- Error propagation per recent commit `3618cb5`.
- Long-running: cancellation, ctx propagation, per-step timeouts.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t1): scene findings"
```

---

### Task 10: Tier 1 — devicectl + drivers

**Files to read:**
- `backend/internal/devicectl/driver.go` (interface)
- `backend/internal/devicectl/factory.go`
- `backend/internal/devicectl/factory_test.go`
- `backend/internal/devicectl/drivers/generic/*.go`
- `backend/internal/devicectl/drivers/homeassistant/*.go`
- `backend/internal/devicectl/drivers/smartthings/*.go`
- `backend/internal/devicectl/drivers/stub/*.go`

**Append findings under:** `## Findings → Phase 2 → Tier 1 → devicectl + drivers`

- [ ] **Step 1: Read interface and each driver implementation**

For each driver, capture:
- Does every command path call `http.Client` with explicit `Timeout` set? (No timeout = bug.)
- Errors wrapped with `fmt.Errorf("...: %w", err)` per `pet/CLAUDE.md` Go style?
- URL validation in `generic_http`: does it reject `127.0.0.1`/`169.254.169.254`/`localhost` to prevent SSRF to cloud metadata? (Likely no — this is a known smell to flag.)
- HA driver: domain → command map vs README claims (switch/light/cover/lock/climate/fan). Any HA domain in README that isn't supported in code?
- SmartThings driver: token rotation? Capability override path tested?
- Idempotency: ON twice → two POSTs (likely; document as Info finding unless device API is non-idempotent).
- Concurrency: is there any per-device mutex? (Likely no; flag.)

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "http\.Client\|Timeout:" backend/internal/devicectl/
grep -rn "%w" backend/internal/devicectl/
grep -rn "169\.254\|metadata\|localhost\|127\.0\.0\.1" backend/internal/devicectl/drivers/generic/
grep -rn "switch\|light\|cover\|lock\|climate\|fan" backend/internal/devicectl/drivers/homeassistant/
grep -rn "panic\|recover" backend/internal/devicectl/
```

- [ ] **Step 3: Walk the spec §5 checklist**

- `Driver` interface: every command has timeout, error wrap, no panic on transport failure.
- `generic_http`: URL validation (no SSRF), Content-Type assumed, response parsing.
- `homeassistant`: token refresh, 401 handling, entity_id check, domain mapping vs README.
- `smartthings`: token rotation, capability mapping, error semantics.
- Idempotency on retry.
- Concurrency: serialized per device or free-for-all.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t1): devicectl + drivers findings"
```

---

### Task 11: Tier 1 — realtime/ws

**Files to read:**
- `backend/internal/realtime/broker.go`
- `backend/internal/realtime/ws/handler.go`
- `backend/internal/realtime/ws/hub.go`
- `backend/internal/realtime/ws/hub_test.go`

**Append findings under:** `## Findings → Phase 2 → Tier 1 → realtime/ws`

- [ ] **Step 1: Read; note auth, backpressure, lifecycle**

Capture:
- Is JWT validated in `handler.go` *before* upgrading? (Common mistake: validate after upgrade.)
- On send to slow client: does the hub `select { case ch <- msg: default: drop }` (non-blocking) or block?
- Goroutine count per connection: read goroutine, write goroutine, hub goroutine — all cleaned on disconnect?
- `SetReadDeadline` + ping/pong loop present?
- `SetReadLimit` (max message size) set?
- Tenant boundary: does the hub broadcast events scoped per-classroom or globally?
- Message schema: is there a `version` field or any schema versioning?

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "Upgrade\|websocket\.Upgrad" backend/internal/realtime/
grep -rn "SetReadDeadline\|SetReadLimit\|Ping\|Pong" backend/internal/realtime/
grep -rn "select\b" backend/internal/realtime/ws/hub.go
grep -rn "version" backend/internal/realtime/
```

- [ ] **Step 3: Walk spec §5 checklist**

- Auth on upgrade; reconnect re-validates.
- Authorization: tenant boundary on messages a teacher sees.
- Backpressure: slow client doesn't block hub goroutine.
- Goroutine leaks on disconnect; ping/pong + read deadline; max message size.
- Schema: version field; forward-compat per `contracts.md`.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t1): realtime/ws findings"
```

---

### Task 12: Tier 2 — classroom + device CRUD

**Files to read:**
- `backend/internal/classroom/*.go`
- `backend/internal/classroom/classroomtest/*`
- `backend/internal/device/*.go`
- `backend/internal/device/devicetest/*`
- `backend/migrations/00002_classrooms.sql`
- `backend/migrations/00003_devices.sql`

**Append findings under:** `## Findings → Phase 2 → Tier 2 → classroom`, `... → device`

- [ ] **Step 1: Read; note authz, cascades, validation**

For classroom:
- List/Get/Update/Delete: each enforces `userID == classroom.ownerID` or membership?
- Soft-delete vs hard-delete; cascade to devices/schedule/scenes (FK ON DELETE behavior in SQL).
- Pagination on list (limit/offset/cursor).

For device:
- Each handler enforces classroom membership for `classroomId` in body/query.
- `driver` field: validated against registered drivers (`factory.IsRegistered(...)`)?
- `config` field is JSON — is it schema-validated per driver, or freeform?
- Offline detection: where is `lastSeenAt` updated; what's the offline threshold?

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "ON DELETE" backend/migrations/
grep -rn "ownerID\|owner_id\|memberOf\|membership" backend/internal/classroom/ backend/internal/device/
grep -rn "driver\b" backend/internal/device/dto.go backend/internal/device/service.go
grep -rn "lastSeenAt\|last_seen_at\|offline" backend/internal/device/ backend/internal/sensor/
```

- [ ] **Step 3: Walk spec §5 checklist (both subsystems)**

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t2): classroom + device findings"
```

---

### Task 13: Tier 2 — sensor + analytics

**Files to read:**
- `backend/internal/sensor/*.go`
- `backend/internal/sensor/sensortest/*`
- `backend/internal/analytics/*.go`
- `backend/migrations/00006_sensors.sql`

**Append findings under:** `... → sensor`, `... → analytics`

- [ ] **Step 1: Read; note ingestion path, history index, aggregation**

Sensor:
- Ingestion: bulk insert or row-by-row? Validation on incoming reading values (sane temp range etc.)?
- History query: `EXPLAIN`-suspect queries? Look for `WHERE device_id = ... AND timestamp BETWEEN ...` and check the migration for matching index.
- Latest-per-device: is it a `DISTINCT ON` or N+1?

Analytics:
- Aggregation: timezone choice when bucketing into days. UTC vs local — pick one and confirm it's correct.
- Energy formula: where is it computed? Units consistent (Wh vs kWh)?
- p95: any indication of slow aggregations?

- [ ] **Step 2: Run targeted greps**

```bash
grep -rn "INSERT\|BULK" backend/internal/sensor/postgres.go backend/internal/sensor/repository.go
grep -rn "DISTINCT ON\|GROUP BY\|date_trunc\|DATE_TRUNC" backend/internal/sensor/ backend/internal/analytics/
grep -rn "energy\|kWh\|wh\b" backend/internal/analytics/
grep -rn "INDEX\|UNIQUE\|CREATE INDEX" backend/migrations/00006_sensors.sql
```

- [ ] **Step 3: Walk spec §5 checklist (both subsystems)**

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t2): sensor + analytics findings"
```

---

### Task 14: Tier 2 — hass (incl. 55s test investigation)

**Files to read:**
- `backend/internal/hass/hass.go`
- `backend/internal/hass/client.go`
- `backend/internal/hass/service.go`
- `backend/internal/hass/service_test.go`
- `backend/internal/hass/handler.go`
- `backend/internal/hass/postgres.go`
- `backend/internal/hass/repo.go`
- `backend/internal/hass/hasstest/*`
- `backend/migrations/00010_hass_config.sql`
- `backend/migrations/00011_hass_refresh_token.sql`

**Append findings under:** `## Findings → Phase 2 → Tier 2 → hass`

- [ ] **Step 1: Read all files; understand onboarding state machine**

Capture:
- Onboarding states: each transition's error path. What happens if HA returns 5xx mid-onboarding?
- Token persistence: stored encrypted in DB? Or plaintext column?
- Self-test endpoint: every check it claims to do — verify it actually does. Compare against README's "per-check table" description.

- [ ] **Step 2: Investigate 55s test runtime**

```bash
(cd backend && go test -v -timeout 120s ./internal/hass/...) 2>&1 | head -80
```

Watch for: `PAUSE`/`PASS` durations, `time.Sleep` in test setup, real HTTP calls (`httptest.NewServer` is fine, but actual outbound to HA isn't).

```bash
grep -rn "time\.Sleep\|httptest" backend/internal/hass/service_test.go backend/internal/hass/hasstest/
```

File a finding regardless of cause: category Tests/Reliability, severity P2, location `backend/internal/hass/service_test.go`, suggested direction = the actual cause + fix sketch.

- [ ] **Step 3: Walk spec §5 checklist**

- Onboarding state machine error paths.
- Token persistence + refresh.
- Self-test accuracy.
- 55s test runtime.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t2): hass findings + 55s test investigation"
```

---

### Task 15: Tier 2 — MQTT integration

**Files to read:**
- `mosquitto/*.conf` (broker config)
- `docker-compose.yml` (mosquitto service definition)
- Search for MQTT client code:

```bash
grep -rln "mqtt\|paho\|mosquitto" backend/ --include="*.go"
```

**Append findings under:** `## Findings → Phase 2 → Tier 2 → MQTT`

- [ ] **Step 1: Read mosquitto config**

```bash
ls mosquitto/
cat mosquitto/*.conf 2>/dev/null
```

Look for: `allow_anonymous`, ACL file references, listener port + auth combination.

- [ ] **Step 2: Read MQTT client integration in Go**

If no MQTT client code exists in `backend/`, that's a finding (Info or P3 — broker is exposed but unused?).

- [ ] **Step 3: Walk spec §5 checklist**

- Broker auth/ACL (anonymous?).
- Topic layout per classroom/tenant.
- Reconnect logic; QoS choices; retained-message cleanup.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t2): MQTT findings"
```

---

### Task 16: Tier 3 — server / httpx / middleware

**Files to read:**
- `backend/internal/server/*.go`
- `backend/internal/platform/httpx/*.go`
- `backend/internal/platform/httpx/middleware/*.go`

**Append findings under:** `## Findings → Phase 2 → Tier 3 → server / httpx / middleware`

- [ ] **Step 1: Skim; note middleware order, CORS, body limits, recover**

Capture:
- Middleware order: panic-recovery should be outermost; auth should be after request-id+logging.
- CORS: origins list — wildcard `*` allowed? With credentials?
- `MaxBytesReader` or equivalent on body? (No limit = DoS via huge bodies.)
- Request-ID middleware: present? Propagated to logs?

- [ ] **Step 2: Run greps**

```bash
grep -rn "Use(\|chi\.Middleware\|Recover\|Cors\|MaxBytes" backend/internal/server/ backend/internal/platform/httpx/
grep -rn "Origin\|cors\.New" backend/internal/server/ backend/internal/platform/httpx/
```

- [ ] **Step 3: Walk spec §5 checklist (Tier 3 subset for these files)**

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t3): server / httpx / middleware findings"
```

---

### Task 17: Tier 3 — platform pieces + main + auditlog + migrations

**Files to read:**
- `backend/internal/platform/i18n/*.go` (+ `backend/locales/`)
- `backend/internal/platform/validation/*.go`
- `backend/internal/platform/postgres/*.go`
- `backend/cmd/server/main.go`
- `backend/internal/auditlog/*.go`
- `backend/migrations/*.sql` (skim)

**Append findings under:** `## Findings → Phase 2 → Tier 3 → platform: i18n / validation / postgres / main / auditlog / migrations`

- [ ] **Step 1: Skim each**

Capture:
- i18n: keys present in all 3 locales (EN/RU/KK)? Diff key sets:

```bash
ls backend/locales/
# For each locale file, count keys:
for f in backend/locales/*.json backend/locales/*.yaml backend/locales/*.yml 2>/dev/null; do
  [ -f "$f" ] && echo "$f: $(grep -c ':' "$f" 2>/dev/null || echo '?')"
done
```

- validation: which DTOs are validated, which aren't? Spot one un-validated handler.
- postgres: pool config (`MaxConns`, `MinConns`, `MaxConnLifetime`)? Reasonable values?
- main.go: signal handling? Graceful shutdown order (HTTP server → drain WS → close DB pool)?
- auditlog: which events are logged? Compare to checklist mentioned in service code.
- migrations: every file has a `-- +goose Down` block.

- [ ] **Step 2: Greps**

```bash
grep -rn "MaxConns\|pgxpool\.Config" backend/internal/platform/postgres/ backend/cmd/server/
grep -rn "SIGINT\|SIGTERM\|signal\.Notify\|Shutdown" backend/cmd/server/
grep -rn "+goose Down" backend/migrations/ | wc -l
ls backend/migrations/ | wc -l   # should equal goose-Down count
```

- [ ] **Step 3: Walk spec §5 checklist (Tier 3 subset)**

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(t3): platform / main / auditlog / migrations findings"
```

---

### Task 18: Mobile — core (api, ws, storage, push, router)

**Files to read:**
- `mobile/lib/core/api/**/*.dart`
- `mobile/lib/core/ws/*.dart`
- `mobile/lib/core/storage/*.dart`
- `mobile/lib/core/push/*.dart`
- `mobile/lib/core/router/*.dart`
- `mobile/lib/core/connection/*.dart`

**Append findings under:** `## Findings → Phase 2 → Mobile → core`

- [ ] **Step 1: Read each; capture envelope handling, ws lifecycle, storage, push, guards**

- API endpoints: every endpoint parses `{data}/{error}` envelope consistently (ref commit `bd4dc0d`).
- WS: reconnect with backoff; dedup of buffered messages on reconnect.
- Storage: only `flutter_secure_storage` — no `SharedPreferences` fallback (per memory).
- Push: how complete is the FCM stub? What's missing for real delivery (token registration, foreground vs background handler)?
- Router: auth guards on protected routes; deep-link handling; back-button traps.

- [ ] **Step 2: Greps**

```bash
grep -rn "shared_preferences" mobile/lib/  # should be 0 hits
grep -rn "FirebaseMessaging\|onMessage\|onBackgroundMessage" mobile/lib/core/push/
grep -rn "redirect\|guard" mobile/lib/core/router/
grep -rn "envelope\|data.*error\|fromJson" mobile/lib/core/api/
```

- [ ] **Step 3: Walk spec §5 mobile checklist (core subset)**

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(mobile): core findings"
```

---

### Task 19: Mobile — features (iot_wizard, notifications, analytics)

**Files to read:**
- `mobile/lib/features/iot_wizard/**/*.dart`
- `mobile/lib/features/notifications/**/*.dart`
- `mobile/lib/features/analytics/**/*.dart`

**Append findings under:** `## Findings → Phase 2 → Mobile → features`

- [ ] **Step 1: Read; capture wizard UX, notification list, analytics charts**

- iot_wizard: OAuth URL extraction logic (ref commit `ac0191f`). Is the host-file requirement explained in-UI for Xiaomi (per README §"Notes on specific vendors")?
- notifications: empty-state widget; pull-to-refresh; error widget shows useful message.
- analytics: chart data flow; what happens on offline / empty / error.

- [ ] **Step 2: Greps**

```bash
grep -rn "homeassistant\.local\|hosts file\|/etc/hosts" mobile/lib/features/iot_wizard/
grep -rn "EmptyState\|NoData\|Error" mobile/lib/features/notifications/ mobile/lib/features/analytics/
```

- [ ] **Step 3: Walk checklist**

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(mobile): features findings"
```

---

### Task 20: Mobile — UX / i18n / accessibility / offline

**Files to read:**
- `mobile/lib/core/i18n/*` and any `*.arb` / `app_localizations*.dart`
- Search for `friendlyError`, `OfflineBanner`, `Semantics`, `MediaQuery.textScaler`:

```bash
grep -rln "friendlyError\|OfflineBanner\|Semantics\|textScaler\|textScaleFactor" mobile/lib/
```

**Append findings under:** `## Findings → Phase 2 → Mobile → UX / i18n / accessibility / offline`

- [ ] **Step 1: i18n key parity**

```bash
ls mobile/lib/core/i18n/  # or wherever .arb files live
# Count keys per locale; diff EN vs RU vs KK to find missing keys
```

- [ ] **Step 2: friendlyError consistency**

```bash
grep -rn "friendlyError" mobile/lib/ | wc -l
# Then sample 5 catch blocks and check they use friendlyError, not raw e.toString()
grep -rn "catch.*\\b" mobile/lib/ | head -20
```

- [ ] **Step 3: Offline banner triggers**

```bash
grep -rn "OfflineBanner\|connectionStatus\|isOnline" mobile/lib/core/connection/
```

- [ ] **Step 4: Accessibility spot-check**

```bash
grep -rn "Semantics(\|semanticLabel:\|excludeSemantics:" mobile/lib/ | wc -l
```

Low count (<10) = finding (P2 MobileUX, "missing accessibility annotations").

- [ ] **Step 5: Walk checklist; commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(mobile): UX/i18n/a11y/offline findings"
```

---

### Task 21: Infra / CI / Supply chain

**Files to read:**
- `Dockerfile` files: `find . -name "Dockerfile" -not -path "*/node_modules/*"`
- `docker-compose.yml`
- `.github/workflows/*.yml`
- `.env.example`
- `.gitignore`

**Append findings under:** `## Findings → Phase 2 → Infra / CI / Supply chain`

- [ ] **Step 1: Dockerfile review**

```bash
find . -name "Dockerfile" -not -path "*/node_modules/*" -not -path "*/vendor/*"
# For each:
# - multi-stage build present?
# - non-root USER set before final CMD?
# - base image pinned by digest or just by tag?
# - any COPY of secrets / .env into image?
```

- [ ] **Step 2: docker-compose review**

```bash
grep -E "healthcheck|depends_on|condition:|ports:|volumes:" docker-compose.yml
```

Check: every service has a healthcheck; dependents use `condition: service_healthy`; ports exposed are minimal.

- [ ] **Step 3: CI workflow review**

```bash
cat .github/workflows/ci.yml
```

Check:
- Are scanners (govulncheck/staticcheck/gosec) wired into CI? (Likely no — finding.)
- Coverage gate? (Likely no — finding.)
- Action versions: pinned by SHA or by tag like `@v4`? Tags are mutable; SHA is best practice but `@v4` is acceptable for pet projects.
- Secrets in workflow: any `${{ secrets.X }}` and is it intentional?

- [ ] **Step 4: .env.example + .gitignore**

```bash
cat .env.example
grep -E "\\.env\\b|secrets|credentials" .gitignore
```

Check: `.env.example` has only placeholders; `.env` is in `.gitignore`.

- [ ] **Step 5: Walk checklist; commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(infra): Dockerfile/compose/CI/env findings"
```

---

## Phase 3 — Cross-cutting passes

### Task 22: Cross-cutting — Contract drift

**Files to read:** every API DTO (`backend/internal/*/dto.go`) vs every mobile model (`mobile/lib/shared/models/`).

**Append findings under:** `## Findings → Phase 3 → Contract drift`

- [ ] **Step 1: Enumerate backend DTOs and mobile models**

```bash
find backend/internal -name "dto.go" | xargs grep -l "json:" | head -20
ls mobile/lib/shared/models/
```

- [ ] **Step 2: Sample 5 DTOs + corresponding mobile models, diff fields**

For each pair (e.g., `backend/internal/device/dto.go` ↔ `mobile/lib/shared/models/device.dart`):
- Extract field names + types from Go (`json:"foo"` tags).
- Extract field names + types from Dart (`@JsonKey('foo')` or `final ... foo`).
- Diff.

Drift = field exists in one but not the other = a finding (Contracts P2).

- [ ] **Step 3: WS message types**

```bash
grep -rn "type.*MessageType\|type.*Event\b" backend/internal/realtime/ws/
grep -rn "enum.*Message\|sealed class.*Event" mobile/lib/core/ws/
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(xc): contract drift findings"
```

---

### Task 23: Cross-cutting — Error handling consistency

**Files to read:** sample 3 random handlers from different packages.

**Append findings under:** `## Findings → Phase 3 → Error handling consistency`

- [ ] **Step 1: Pick 3 handlers at random**

E.g., `backend/internal/classroom/handler.go`, `backend/internal/scene/handler.go`, `backend/internal/device/handler.go`. Pick one error path in each.

- [ ] **Step 2: Trace each path**

For each: starting at the error return in `service.go`, follow back through `handler.go` to the response written. Capture:
- Is the original error wrapped or replaced?
- Does the response leak internal details (stack trace, SQL error string, file paths)?
- Is the error logged once, multiple times, or not at all?
- Is the HTTP status code correct (4xx for user error, 5xx for server)?

- [ ] **Step 3: Look for swallowed errors**

```bash
grep -rn "_, err := \|err :=" backend/internal/ | grep -v "if err" | grep -v "_test.go" | head -30
```

(Heuristic — manual review needed; not every match is a swallow.)

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(xc): error handling consistency findings"
```

---

### Task 24: Cross-cutting — Secret scan + PII in logs

**Append findings under:** `## Findings → Phase 3 → Secret scan`, `... → PII in logs`

- [ ] **Step 1: Secret scan**

```bash
grep -rEn "sk-|api_key|password\s*=|secret\s*=" backend/ mobile/lib/ docker-compose.yml --include="*.go" --include="*.dart" --include="*.yml" --include="*.yaml" --include="*.env*" --exclude-dir=vendor 2>/dev/null | grep -vE "^\s*//|^\s*#|_test\.go|/locales/|placeholder|example|Placeholder|Example" | head -50
```

Every hit is a finding (P0 if real, P3 if false-positive variable name).

- [ ] **Step 2: PII in logs (backend)**

```bash
grep -rn "log\.\|logger\." backend/internal/ --include="*.go" | grep -iE "email|phone|iin|password|name|fullname" | grep -v "_test.go" | head -30
```

Each match where user-supplied data is interpolated into a log line = finding (Security/Observability P2 per `pet/CLAUDE.md` §11).

- [ ] **Step 3: PII in logs (mobile)**

```bash
grep -rn "print(\|debugPrint(\|log\.\|logger\." mobile/lib/ | grep -iE "email|phone|password|token" | head -20
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(xc): secret scan + PII in logs findings"
```

---

### Task 25: Cross-cutting — Metrics/traces presence

**Append findings under:** `## Findings → Phase 3 → Metrics/traces presence`

- [ ] **Step 1: Count metric calls**

```bash
grep -rn "prometheus\|otel\|metrics\.\|tracer\.\|instrument" backend/ --include="*.go" | wc -l
grep -rn "prometheus\|otel\|metrics\.\|tracer\.\|instrument" backend/ --include="*.go" | head -20
```

Likely 0 — that's the finding (one P2 Observability finding for the whole backend).

- [ ] **Step 2: Health/readiness depth**

```bash
grep -rn "/health\|Healthz\|readiness\|liveness" backend/internal/ backend/cmd/
```

If there's a healthz that returns 200 unconditionally (no DB ping, no HA ping), that's a P2 Observability finding per `pet/CLAUDE.md` §13.

- [ ] **Step 3: Structured logging fields**

Sample 5 log lines across services; do they include `requestID` / `userID` / `classroomID`? If most don't, finding.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(xc): metrics/traces/health findings"
```

---

## Phase 4 — Synthesize & recommend

### Task 26: Write executive summary, top-5 risks, cross-cutting observations

**Files:**
- Modify: `docs/superpowers/audits/2026-05-01-deep-audit.md` (Executive summary, Cross-cutting observations sections)

- [ ] **Step 1: Count findings by severity**

Open the audit doc. Count `**Severity:** P0|P1|P2|P3|Info` occurrences. Format:

```markdown
## Executive summary

**Total findings:** N
- **P0 (Critical):** X
- **P1 (High):** Y
- **P2 (Medium):** Z
- **P3 (Low):** W
- **Info:** K

**Top 5 risks** (severity + impact ranked):
1. [F-NNN] — <one-line title> — <why it's #1>
2. [F-NNN] — ...
3. [F-NNN] — ...
4. [F-NNN] — ...
5. [F-NNN] — ...

**Recommended next 3 specs:** see `next-specs.md`.
```

Replace the placeholder under `## Executive summary`.

- [ ] **Step 2: Fill cross-cutting observations**

Replace `_Filled by Phase 4._` under `## Cross-cutting observations` with three short paragraphs:

```markdown
## Cross-cutting observations

### Contract drift
<2-4 sentences summarizing total drift count, worst offenders, pattern (rename? type change? missing field?).>

### Observability gap
<2-4 sentences: how many metrics/traces? Health depth? Structured-log gaps?>

### Test gap
<2-4 sentences: per-package coverage range; which packages are most exposed (low coverage + Tier 1 risk).>
```

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit.md
git -c commit.gpgsign=false commit -m "audit(phase4): executive summary + cross-cutting observations"
```

---

### Task 27: Cluster findings into next-specs.md

**Files:**
- Modify: `docs/superpowers/audits/2026-05-01-deep-audit/next-specs.md`

- [ ] **Step 1: Cluster findings**

Group findings by theme (not by severity). Common clusters that emerge:
- "Observability spec" — every metrics/traces/health/structured-log finding.
- "Security hardening spec" — JWT/refresh, rate-limit, MQTT auth, gosec hits, secret scan hits.
- "Test coverage spec" — every Tests-category finding + coverage <30% packages.
- "Reliability spec" — timeouts, idempotency, graceful shutdown, reconnect, backpressure.
- "Mobile completeness spec" — FCM, error UX, offline, accessibility, i18n parity.
- "Contract versioning spec" — every Contracts-category finding.

Pick the top 3–5 clusters by aggregate weight (severity × count).

- [ ] **Step 2: Write next-specs.md**

Replace the skeleton with:

```markdown
# Downstream specs — derived from 2026-05-01 deep audit

Each cluster below should become its own brainstorm → spec → plan cycle.
Findings are linked by `F-NNN`.

## Spec 1: <Name> (priority: P0/P1)
**Scope:** <1-2 sentences>
**Findings covered:** F-NNN, F-NNN, F-NNN, ...
**Why this first:** <1 sentence>
**Estimated effort:** <S/M/L weeks>

## Spec 2: <Name>
... (same structure) ...

## Spec 3: <Name>
... (same structure) ...

## Deferred (lower priority, capture-only)
- F-NNN, F-NNN — small/local fixes that can be bundled into a "quick wins" PR without a full spec.
- F-NNN — needs more investigation; not yet ready for a spec.
```

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/audits/2026-05-01-deep-audit/next-specs.md
git -c commit.gpgsign=false commit -m "audit(phase4): cluster findings into next-specs.md"
```

---

### Task 28: Final verification + done-criteria check

**Files:** No changes; verification only.

- [ ] **Step 1: Verify done criteria from spec §9**

Open the audit doc. Confirm:
- All Tier 1 subsystems (auth, notification, schedule, scene, devicectl, realtime/ws) have either a finding or a "no findings" note under their heading.
- Tool output appendix has `Ran?` filled for every tool (yes or "skipped — reason").
- `next-specs.md` lists ≥3 candidate specs.
- Executive summary fits on one screen (~30 lines or fewer).

- [ ] **Step 2: Re-run minimal smoke**

```bash
git log --oneline | head -30
ls docs/superpowers/audits/2026-05-01-deep-audit/raw/
wc -l docs/superpowers/audits/2026-05-01-deep-audit.md
```

Expected: `wc -l` returns >300 lines; raw/ contains the 8–11 expected files; git log shows ~28 audit commits.

- [ ] **Step 3: Final commit (only if any tweaks needed during verification)**

```bash
git status
# If clean, no commit needed.
# If tweaks made:
git add docs/superpowers/audits/
git -c commit.gpgsign=false commit -m "audit: final verification fixes"
```

- [ ] **Step 4: Print summary to user**

Output the executive summary block + the next-specs.md content for user review. State which downstream spec the user should brainstorm next.

---

## Self-Review

**Spec coverage check:**
- Spec §3 Phase 1 (automated scan) → Tasks 1–5. ✓
- Spec §3 Phase 2 (subsystem deep-read) → Tasks 6–21. ✓
- Spec §3 Phase 3 (cross-cutting) → Tasks 22–25. ✓
- Spec §3 Phase 4 (synthesize) → Tasks 26–27. ✓
- Spec §5 Tier 1 subsystems (auth, notification, schedule, scene, devicectl, ws) → one task each (6–11). ✓
- Spec §5 Tier 2 (classroom, device, sensor, analytics, hass, MQTT) → Tasks 12–15. ✓
- Spec §5 Tier 3 (server/httpx/i18n/etc.) → Tasks 16–17. ✓
- Spec §5 Mobile → Tasks 18–20. ✓
- Spec §5 Infra → Task 21. ✓
- Spec §5 cross-cutting (5 sub-passes) → Tasks 22–25. ✓
- Spec §6 deliverables (audit doc + raw/ + next-specs.md) → Tasks 0, 26, 27. ✓
- Spec §9 done criteria → Task 28 explicit verification. ✓

**Placeholder scan:** No "TBD"/"TODO" inside task steps (the `_Filled by Task 5._` placeholders inside the audit doc skeleton are intentional and explicitly named to be filled by named tasks). ✓

**Type/name consistency:** Audit doc path is consistent (`docs/superpowers/audits/2026-05-01-deep-audit.md`). Raw dir consistent (`docs/superpowers/audits/2026-05-01-deep-audit/raw/`). Finding ID format `F-NNN` consistent. ✓

**Stop rules surfaced:** Yes — header section + each task implicitly relies on them. ✓

Plan ready.
