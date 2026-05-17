# Final Audit Plan — smartclass

**Status:** draft v1
**Owner:** Arsen
**Trigger:** to be executed once PRs #3–#6 are merged into `main`
**Estimated effort:** 4–6 focused hours, single iteration.

---

## 0. Why this audit, why now

In the bug-sweep iteration (PR #2) we landed ~80 fixes across mobile and backend over
five QA passes. The follow-up enhancement PRs add real test coverage (integration tests)
and feature surface (push notifications, offline cache, stricter CI). Each new surface
is a new attack surface for bugs. This audit closes the iteration by checking the
**combined** state of the codebase against a fresh, unbiased rubric instead of incremental
"did we fix what we said we'd fix" passes.

The aim is not to find more bugs to chase — it's to **declare a known-good baseline** for
the next phase of work.

---

## 1. Scope

In-scope:
- `mobile/` Flutter app — full tree under `lib/`, `test/`, `pubspec.*`.
- `backend/` Go service — full tree under `cmd/`, `internal/`, `migrations/`, `locales/`.
- `.github/workflows/` — CI pipeline correctness.
- `docker-compose.yml`, `Makefile`, top-level scripts.

Out-of-scope:
- Performance load testing (separate effort).
- Penetration testing against a live cluster (no live cluster yet).
- Mobile golden-image visual regression (separate effort).
- Backend dependency upgrades (file a follow-up if any flagged).

---

## 2. Pre-flight (15 min)

Confirm baseline before opening any source file:

```bash
# All four PRs merged into main:
gh pr list --state merged --base main | grep -E "#3|#4|#5|#6"

# Local main is at the squash-merge commit of #6:
git checkout main && git pull --ff-only

# Tooling versions match CI:
go version            # expect 1.25.x
flutter --version     # expect 3.41.7
golangci-lint --version  # expect v2.5.x
```

Smoke checks (must all pass before doing the audit proper — if any fails, fix
the regression first, then resume):

```bash
# Backend
cd backend
go mod tidy && git diff --exit-code go.mod go.sum
go vet ./...
golangci-lint run ./...
go test -race -count=1 ./...
go test -tags=integration -race -count=1 ./...   # needs Docker
go test -tags=slow -race -count=1 ./internal/hass/...

# Mobile
cd ../mobile
flutter pub get
flutter analyze
flutter test --coverage
```

If any of these fail, **stop here**, fix the regression in a hotfix branch,
re-merge, and restart pre-flight.

---

## 3. The eight audit lanes

The audit splits into eight independent lanes. Lanes 1–4 are static; lanes 5–8
are dynamic/manual. Run static lanes in parallel via subagents, then collect
results and run dynamic lanes sequentially.

### Lane 1 — Contract integrity (mobile ↔ backend)

**What:** Every HTTP endpoint registered on the backend has a matching call on
the mobile side with compatible DTO shape. Every JSON field name, type, nullability,
and enum value matches end-to-end.

**Method:**
- Enumerate routes: `grep -RE "r\\.(Get|Post|Put|Patch|Delete)\\(" backend/internal/`
  and `grep -RE "_client\\.(get|post|put|patch|delete)\\(" mobile/lib/core/api/endpoints/`.
- Build a matrix: route × backend DTO × mobile model × known consumers.
- For each row, verify field-by-field with the actual struct definitions (no
  assumptions from API docs that may be stale).
- Special focus on integer-vs-string IDs (we burned twice on this), datetime
  encoding (RFC3339), enum constants, and nullable-but-always-present strings.

**Deliverable:** spreadsheet-style table with one row per endpoint and a
"green / yellow / red" status. Reds are fixed inline; yellows are filed.

**Owner:** code-reviewer subagent (adversarial), then qa-engineer cross-check.

### Lane 2 — WebSocket protocol end-to-end

**What:** Mobile WS client behaviour against backend `/api/v1/ws` is correct
across ticket-issue, upgrade, subscribe, event-receive, reconnect-with-fresh-ticket,
and shutdown paths.

**Method:**
- Manual: with the backend running locally, connect the mobile app to a classroom,
  kill the backend, restart it, confirm the mobile reconnects with a fresh ticket
  (M-001 + C-006 territory). Watch network logs.
- Code review: WsClient state machine — every branch of `_scheduleReconnect`,
  every callback. The exponential backoff cap. The max-retries.
- Topic dispatch: every event type backend can publish is handled mobile-side.
  Specifically — confirm `scenes.*` events now refresh `sceneListProvider` (M-002).

**Deliverable:** sequence diagrams (sketches OK) for: cold connect, mid-session
reconnect, backend restart, ticket expiry, max-retries cap.

**Owner:** qa-engineer.

### Lane 3 — Concurrency / shutdown / lifecycle

**What:** No goroutine leaks, no data races, no graceful-shutdown gaps. Mobile:
no provider lifecycle bugs (autoDispose, ref.read/watch misuse).

**Method:**
- Backend: re-run `go test -race -count=1 ./...` and confirm 0 races. Walk the
  shutdown sequence in `cmd/server/main.go` line-by-line: signal handler →
  rate-limiter Stop → http.Shutdown → audit-log FlushSync → DB pool Close.
  Verify every long-running goroutine has a cancellation path (audit-log, hass
  bootstrap retry, refresh-token purge, rate-limiter cleanup, WS hub).
- Mobile: grep for `ref.read` inside `build` (Riverpod anti-pattern). Grep for
  `setState` without `mounted` check before. Grep for `TextEditingController`
  and confirm each has a `dispose` partner.

**Deliverable:** confirmed-clean list per long-lived goroutine + per controller.

**Owner:** backend-engineer + flutter-engineer (split).

### Lane 4 — Security pass

**What:** Standard OWASP-ish quick review focused on this codebase.

**Method:**
- AuthZ: every authenticated route enforces the right role; sensitive routes
  (admin-only) actually check it. Bearer token validation. JWT signing key
  source (env, rotatable).
- Input validation: every `httpx.DecodeJSON` is followed by `validate.Struct`.
- SQL: no string-concat queries. Every parameterised query uses `$N`.
- File / path: no user input concatenated into filesystem paths.
- Secrets: nothing committed. `grep -RE "sk-|api_key|password\\s*=\\s*\"" backend/ mobile/`.
- Mobile token storage: secure storage (not SharedPreferences) for refresh tokens.
- Mobile WebSocket: `wss://` enforced in prod URL config (no `ws://`).
- Push notification payload: no PII (faces, IINs, full names) embedded in
  notification body.
- CORS: backend whitelist matches actual frontends.
- Rate limiting: trusted-proxies CIDR set in deployment env (C-021).
- Headers: security headers (CSP, HSTS) — if missing, file follow-up.

**Deliverable:** OWASP-style table; HIGH findings → fix inline; MEDIUM → follow-up.

**Owner:** security-engineer subagent.

### Lane 5 — Migration / data integrity

**What:** Migrations apply cleanly from scratch and roll back cleanly. No
out-of-order issues. Indexes match the actual query patterns.

**Method:**
- Spin up a fresh testcontainer Postgres. Apply migrations 1..N forward. Confirm
  Goose history matches `pg_tables`.
- Down: roll back migration N → N-1 → ... → 0. Confirm clean state. (Some Down
  blocks are intentionally no-op — those are documented; verify the doc.)
- Re-apply forward. Re-run `go test -tags=integration`.
- Scan `EXPLAIN` plans for the top 5 most-called repo queries; confirm index
  use.

**Deliverable:** migration matrix + index coverage report.

**Owner:** database-engineer (via Go scripts) + backend-engineer.

### Lane 6 — Mobile UX / golden-path smoke

**What:** With the backend running and seeded, walk the golden path. The build
runs and the user-visible flows do what they should.

**Method:**
- `make seed` to load fixture data.
- Run the Flutter app against the local backend.
- Walk: register → login → create classroom → invite → add device → run scene →
  see notification → switch language to RU then KK and re-walk → toggle airplane
  mode mid-flow and observe offline cache banner → reconnect.
- Test on physical Android device if available, otherwise emulator.

**Deliverable:** numbered screen-by-screen notes. Anything that "should not
have shipped" → fix or file.

**Owner:** qa-engineer (manual).

### Lane 7 — Observability dry-run

**What:** Confirm what we'd actually see in production logs when something
goes wrong.

**Method:**
- Inject 5 failure scenarios in a local run:
  1. Postgres down (kill the docker-compose Postgres mid-request).
  2. FCM credentials invalid (set a bogus FIREBASE_SERVICE_ACCOUNT_JSON).
  3. WS ticket expired (race the issue and the upgrade).
  4. HASS upstream 500 (mock the HA URL).
  5. Refresh-token replay (call refresh with an already-used token).
- For each: scrape `cmd/server` logs and verify each event is logged at the
  correct level with enough context to diagnose. No PII in any line. No JWT
  fragments.

**Deliverable:** failure scenario × log line × verdict (clear / vague / missing).

**Owner:** backend-engineer or sre-engineer.

### Lane 8 — CI / pipeline self-check

**What:** Does the pipeline actually catch what we think it catches?

**Method:**
- Intentionally introduce a synthetic regression on a sandbox branch and
  confirm CI fails. One per category:
  - `golangci-lint`: add `var unused int` to a Go file.
  - `go test -race`: add a quick race in a test (then revert).
  - `flutter analyze`: add `var foo;` to a Dart file.
  - `flutter test`: change an `expect` to fail.
  - `integration` job: add a SQL syntax error to a Up migration.
- Confirm each branch fails fast and clearly.
- Confirm coverage gates fire when coverage drops below 30%.
- Confirm `go mod tidy is clean` step fails on dirty go.sum.

**Deliverable:** pass/fail matrix per safety net.

**Owner:** devops-engineer.

---

## 4. Aggregation & exit criteria

After all eight lanes complete:

1. Collect findings into a single Markdown table: lane × severity × inline-fix-or-followup.
2. **HIGH** = block release; fixed inline in a new branch `audit/final-fixes-YYYY-MM-DD`.
3. **MEDIUM** = followup ticket filed, but allowed to ship.
4. **LOW** = filed and triaged later.
5. Once all HIGHs are fixed and CI is green on the audit-fixes PR, the audit
   exits.

Exit verdict shape:

```
## Final audit verdict
- Lanes 1–8 executed: YES / NO
- HIGH findings: N (all fixed inline)
- MEDIUM findings: N (filed as #...#)
- LOW findings: N (filed as #...#)
- Combined CI green on main: YES / NO
- Verdict: READY-FOR-NEXT-PHASE / NEEDS-MORE-WORK
```

---

## 5. Known-unknowns going in

Bugs / risks already known that the audit should **explicitly confirm or refute**:

- **C-014 (rejected as a bug)** — token-refresh storm. Confirm again with a
  concurrent-401 stress test that doesn't deadlock.
- **C-018** — DB persist failure during HASS token refresh retry sets
  `bootstrapErr`. Verify retry loop is actually exercised in the audit's failure
  injection (lane 7).
- **M-202 (fixed)** — context cancellation during HASS retry. Verify by
  cancelling mid-retry in an integration test.
- **G-110** — WS principal Role passed from ticket. Verify with an admin login
  + WS connection: does the admin see admin-only classroom topics?
- **Plural forms (M-005, C-017)** — verify RU/KK rendering with `count = 0, 1, 2, 5, 21`
  on actual UI.
- **Offline cache TTL** — confirm the banner appears past 1h and the data still
  renders.
- **FCM ErrInvalidToken path** — verify a known-bad token is deleted from DB
  after one failed Send.
- **`refresh_tokens` purge goroutine** — verify it runs and deletes expired rows
  after the configured interval; verify it doesn't block shutdown.

---

## 6. Anti-goals (do not do during the audit)

- Don't refactor. Audit ≠ rewrite. If you spot architectural cleanups, file them
  for a separate phase.
- Don't upgrade deps. CVE-driven upgrades only; everything else is a separate PR.
- Don't add new features. The audit is a quality gate, not a backlog drain.
- Don't extend test coverage broadly. Add tests only for the specific bugs
  you find during the audit.

---

## 7. Execution checklist (copy into a tracking issue)

- [ ] Pre-flight smoke checks pass
- [ ] Lane 1 — Contract integrity report
- [ ] Lane 2 — WS protocol report + sequence diagrams
- [ ] Lane 3 — Concurrency / lifecycle clean list
- [ ] Lane 4 — Security pass table
- [ ] Lane 5 — Migration / index report
- [ ] Lane 6 — Mobile UX smoke notes
- [ ] Lane 7 — Observability dry-run results
- [ ] Lane 8 — CI safety net matrix
- [ ] Aggregated findings table compiled
- [ ] HIGH findings fixed inline in `audit/final-fixes-YYYY-MM-DD`
- [ ] MEDIUM/LOW followups filed
- [ ] Final verdict declared
- [ ] PR merged, branch deleted
