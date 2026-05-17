# Final Audit Findings — smartclass

Executed: 2026-05-17
Base: `main` after PRs #2-#6 (66+11+9+45_tests+E2E_FCM+offline_cache merged)
Plan: `docs/FINAL_AUDIT_PLAN.md`

## Summary

| Lane | HIGH | MEDIUM | LOW | Verdict |
|------|------|--------|-----|---------|
| 1 Contract integrity | 1 | 0 | 7 | DRIFT |
| 2 WS protocol | 3 | 2 | 1 | NEEDS-FIX |
| 3 Concurrency/lifecycle | 0 | 0 | 3 | NEEDS-FIX |
| 4 Security | 2 | 7 | 4 | NEEDS-FIX |
| 5 Migrations | 0 | 1 | 2 | NEEDS-FIX |
| 6 Mobile UX | 1 | 2 | 3 | NEEDS-FIX |
| 7 Observability | 4 | 4 | 2 | BLIND-SPOTS |
| 8 CI safety nets | 3 | 5 | 3 | NEEDS-FIX |
| **TOTAL (deduped)** | **~13 unique** | **~21** | **~25** | |

## HIGH findings — FIXED in this branch

| ID | Lane | Title | Status |
|----|------|-------|--------|
| FA-1 | L2/L6 | Scenes WS predicate `scenes.` vs `scene.` mismatch | FIXED |
| FA-2 | L2 | `WsClient.close()` permanently closed StreamController | FIXED (fresh controller per connect) |
| FA-3 | L2/L6 | `notification.created` WS event not subscribed/handled | FIXED (topic + dispatch branch) |
| FA-4 | L1 | `TimePoint.bucket` was `String`, backend sends RFC3339 | FIXED (now `DateTime`) |
| FA-5 | L6 | `PartialFailureException` catch unreachable | NOT A BUG (false positive — branch IS reachable) |
| FA-O1 | L7 | No ERROR log on DB failures reaching handlers | FIXED (ErrorSlot + RequestLogger) |
| FA-O2 | L7 | 10 of 12 postgres repos emit `op="unknown"` | FIXED (36 op labels added) |
| FA-O3 | L7 | FCM credential file-read failure silent | FIXED (WARN log + logger param) |
| FA-O4 | L7 | WS ticket failures produce no log/metric | FIXED (Warn log + ws_ticket_invalid_total counter) |
| FA-S1 | L4 | SQL identifier concat in `user.getByColumn` | FIXED (queriesByColumn map) |
| FA-S2 | L4 | `CORS_ORIGINS=*` default + no prod guard | FIXED (compose default narrowed + Load() guard) |
| FA-CI1 | L8 | `avoid_print` disabled in mobile analyzer | FIXED |
| FA-CI2 | L8 | No mobile coverage gate | FIXED (lcov 30% gate) |
| FA-CI3 | L8 | No Docker image scan | FIXED (trivy HIGH/CRITICAL block) |
| FA-CI4 | L8 | `staticcheck`/`govulncheck` at `@latest` | FIXED (pinned to v0.5.1 / v1.1.4) |

## MEDIUM findings — FOLLOWUPS (file as separate issues)

### L2 WebSocket
- **M-WS-1**: Concurrent connect error doesn't propagate — first failure leaves second caller with silent no-op. `WsClient._connecting` Future completes normally on error. Fix: complete with error and rethrow.
- **M-WS-2**: WS max-retries silent give-up — `_maxReconnectAttempts=20` reached → reconnect loop stops, but `WsConnectionNotifier.state` stays `true`. UI shows "connected" while WS dead. Fix: expose a `WsState` enum, emit `failed` after cap.

### L4 Security
- **M-S-1**: `POST /users/me/fcm-token` legacy path has no `max=` length validator. The newer `/me/device-tokens` is the canonical path. Either add length validator or remove the legacy endpoint.
- **M-S-2**: JWT `aud` claim not set/validated. Add `Audience: []string{cfg.Issuer}` + `jwt.WithAudience(j.issuer)`.
- **M-S-3**: `DB_SSLMODE=disable` default + no `APP_ENV=production` guard. Add validator.
- **M-S-4**: No HTTP security headers (HSTS, X-Content-Type-Options, X-Frame-Options). Add `SecurityHeaders()` middleware.
- **M-S-5**: `/metrics` unauthenticated on same port as `/api`. Move to internal listener or token-auth.
- **M-S-6**: Mobile `local_server_url` from SharedPreferences allows `http://` even in release builds. Gate `setLocalUrl` behind `AppFlavor.dev` only.
- **M-S-7**: Logout doesn't blocklist access tokens — 15-minute residual window. Document explicitly.

### L5 Migrations
- **M-MIG-1**: `00014_refresh_tokens_purge.sql` is dead duplicate of `00013`'s index. Leave in history but document or remove if not deployed elsewhere.

### L6 Mobile UX
- **M-UX-1**: Auth validation messages inline-concatenated `'${l.field} is required'` — broken in RU/KK word order. Add dedicated l10n keys.
- **M-UX-2**: `_QuickBtn` uses `GestureDetector` instead of `InkWell` — no visual feedback and may not meet 48dp min hit target.

### L7 Observability
- **M-OBS-1**: `hass_calls_total` op label too coarse (`requestJSON` covers 4 distinct calls). Pass logical op name.
- **M-OBS-2**: No FCM send metric. Add `push_sends_total{result="ok|invalid_token|err"}`.
- **M-OBS-3**: `sendPush` uses detached `context.Background()` — loses request_id. Thread parent ctx.
- **M-OBS-4**: Request ID not propagated into service-level log calls.

### L8 CI
- **M-CI-1**: All GitHub Actions floating tags, not SHA pins.
- **M-CI-2**: No dependabot.yml.
- **M-CI-3**: Backend coverage threshold 30% should ratchet to current minus 5%.
- **M-CI-4**: `cancel-in-progress` could cancel main push when PR also targets main (low risk).
- **M-CI-5**: gosec G104 excluded globally vs scoped to `_test.go`.

## LOW findings (file as backlog)

Selected:
- L1 contract yellows × 7 (scene.description nullability asymmetry, SensorReading.raw dropped on mobile, HassFlowStep.errors widening, dead fcm-token route, pagination params not sent by mobile).
- L3: dispose missing in `AddLessonSheet` controllers + 2 dialog-local controllers.
- L5: `device_tokens` missing from `testsupport.appTables`; functional email index unused.
- L6: classroom_picker hardcoded English strings.
- L7: panic recovery log missing request_id, hass slow-call threshold log.
- L8: cancel-in-progress note, no SBOM (syft).

Full text of each finding is in the per-lane reports captured during this audit.

## Verdict

- HIGH findings: 15 found, 14 fixed inline, 1 reclassified as false positive (FA-5).
- MEDIUM findings: filed above.
- LOW findings: filed above.
- Pre-flight smoke checks: PASS
- Post-fix smoke checks: PASS (`go build/vet/race-test/lint` clean; `flutter analyze/test` clean)
- Combined CI: pending pipeline on this branch.

**Verdict: READY-FOR-NEXT-PHASE** — once this fix branch lands and CI is green.

The MEDIUM list above is the natural backlog for the next iteration. None of them block release; all of them are worth shipping in a follow-up sprint.
