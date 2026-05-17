# Test Coverage Hardening — Design Spec

**Date:** 2026-05-01
**Topic:** Critical-path test coverage improvements: hass slow-test isolation, handler-level tests for the highest-risk subsystems, schedule overlap correctness, CI coverage gate.
**Status:** Approved (ready for plan)
**Source:** `docs/superpowers/audits/2026-05-01-deep-audit.md` findings F-006 (handler/service coverage), F-023 (hass 55s test runtime).

## 1. Purpose

Backend total coverage is 29.8% after the observability spec landed (was 26.1% pre-audit). Most service packages still sit at 13–26%; HTTP handler layers across notification, classroom, scene, schedule, device, sensor, analytics are at 0% — the entire request-validation + error-mapping path is untested. Refactoring those subsystems carries hidden risk.

Separately, the `hass` package's tests take 55 seconds — almost the entire `go test ./...` runtime. Every developer's local feedback loop and every CI build pays that tax.

This spec doesn't try to push the whole codebase to 60% — that's multi-day effort and weak ROI for low-traffic packages. Instead it picks the four highest-risk subsystems (notification, scene, schedule, classroom) and brings their handlers + service edge cases to a level where future refactors are safe, isolates hass slow tests behind a build tag, and adds a CI gate so coverage never regresses below 35%.

## 2. Scope

**In scope**
- **hass slow-test isolation** — split tests that depend on `BootstrapWithRetry`'s 3+s backoff into a `//go:build slow` file; default `go test` skips them, CI runs them in a separate dedicated step.
- **scene handler tests** (`backend/internal/scene/handler_test.go`) — happy path Run, partial-fail, full-fail, authz reject (non-member), scene not found.
- **schedule overlap correctness** (`backend/internal/schedule/service_overlap_test.go`) — table-driven cases for adjacency, edit-overlap-with-self, same-minute boundary, end ≤ start rejection.
- **schedule handler tests** (`backend/internal/schedule/handler_test.go`) — list/create/update/delete + status-code edges (400/401/403/404/409).
- **notification trigger expansions** (extend `trigger_test.go`) — dedup window, threshold boundaries, missing-FCM-token path.
- **notification handler tests** (`backend/internal/notification/handler_test.go`) — list/MarkRead/MarkAllRead, pagination clamp behavior, authz scope.
- **classroom handler tests** (`backend/internal/classroom/handler_test.go`) — CRUD as admin vs. teacher, list pagination, member assign/unassign.
- **CI coverage gate** — `.github/workflows/ci.yml` parses `go tool cover -func` total and fails the build when total < 35%. Threshold tunable via repo variable / inline constant.

**Out of scope (deferred)**
- analytics handler/service tests — low traffic, separate spec.
- sensor handler tests — service tests already exist; handler is a thin pass-through.
- device CRUD handler tests — bigger surface, separate spec.
- internal/server route-wiring tests beyond the existing /metrics smoke test.
- pgx mocking — all tests use the existing in-memory repos in `*test/memrepo.go`.
- Integration / E2E tests via docker-compose — separate spec.
- Mobile coverage — Flutter is not the bottleneck.

## 3. Architecture

### File layout

```
backend/
├── .github/workflows/ci.yml                      ← +1 step "Coverage gate"
├── internal/hass/
│   ├── service_test.go                           ← keep ONLY fast tests
│   ├── service_slow_test.go (NEW)                ← `//go:build slow` — backoff/poll-bound tests
│   └── service.go                                ← optional clock injection seam (only if needed)
├── internal/scene/handler_test.go (NEW)          ← 5 tests
├── internal/schedule/
│   ├── handler_test.go (NEW)                     ← 5 tests
│   └── service_overlap_test.go (NEW)             ← 6 table-driven cases
├── internal/notification/
│   ├── handler_test.go (NEW)                     ← 4 tests
│   └── trigger_test.go                           ← +4 cases
└── internal/classroom/handler_test.go (NEW)      ← 5 tests
```

### Test-construction pattern

Every new handler test follows this shape (consistent with existing `auth/handler_test.go`):

```go
func newScheduleHandler(t *testing.T) (*schedule.Handler, *schedule.Service) {
    t.Helper()
    classroomRepo := classroomtest.NewMemRepo()
    classroomSvc := classroom.NewService(classroomRepo)
    repo := scheduletest.NewMemRepo()
    svc := schedule.NewService(repo, classroomSvc)
    bundle := i18n.NewBundle(i18n.EN)
    require.NoError(t, bundle.LoadDir(localesDir(t)))
    return schedule.NewHandler(svc, validation.New(), bundle), svc
}

func TestHandler_CreateLesson_RejectsOverlap(t *testing.T) {
    metrics.Reset()
    h, svc := newScheduleHandler(t)
    seedClassroomAndLesson(t, svc, ...)

    rec := doRequest(t, h, http.MethodPost, "/schedule/...", body)
    assert.Equal(t, http.StatusConflict, rec.Code,
        "...explanation of why...")
}
```

`metrics.Reset()` at the top of every handler test isolates the metric counters that handlers now increment (auth/notification/scene); without it, counter values bleed across tests.

### hass slow-test split strategy

The existing `service_test.go` file has ~10 test functions; ~7 of them call `BootstrapWithRetry` or its derivatives, each waiting at least one 3-second backoff cycle.

The split:
1. Move slow tests (the ones that wait on real `time.After`) into `service_slow_test.go` with `//go:build slow` at the top.
2. Default `go test ./internal/hass/...` runs only the fast tests — sub-1s.
3. CI gets a new step `go test -tags=slow ./internal/hass/...` that runs slow tests in a separate ~1m job. Local devs can still run them with `go test -tags=slow ./...`.

If a test is slow only because of one obvious sleep, prefer fixing the test (clock injection) over moving it. The build-tag split is for tests that genuinely exercise the retry/backoff behavior — those are integration-quality and benefit from being separable.

### CI coverage gate

```yaml
- name: Coverage gate
  run: |
    pct=$(go tool cover -func=coverage.out | awk '/^total/ {print int($3)}')
    echo "Total coverage: $pct%"
    if [ "$pct" -lt 35 ]; then
      echo "::error::Total coverage $pct% is below threshold 35%"
      exit 1
    fi
```

Threshold lives inline. Bumping it (after future specs raise coverage) is a one-line edit.

## 4. Test catalogue

### scene handler (5 tests)

- `Run_Ok_200` — valid scene, all steps succeed
- `Run_PartialFail_200WithStepResults` — 2 of 3 steps fail; response includes per-step results
- `Run_AllStepsFail_502` — every step fails; status reflects upstream failure
- `Run_NonMemberClassroom_403` — caller not a member of scene's classroom
- `Run_NotFound_404` — scene id doesn't exist

### schedule overlap correctness (6 cases, table-driven)

Each case sets up a classroom with an existing lesson (Mon 09:00-10:00) and asserts on the result of creating/updating a second lesson:

| New lesson | Day | Start | End | Expected |
|---|---|---|---|---|
| edge-touch | Mon | 10:00 | 11:00 | OK (half-open boundary) |
| same-minute | Mon | 09:30 | 10:30 | conflict |
| backward boundary | Mon | 08:00 | 09:00 | OK |
| end ≤ start | Mon | 09:00 | 09:00 | invalid |
| different day | Tue | 09:00 | 10:00 | OK |
| edit-self (no shift) | Mon | 09:00 | 10:00 | OK (updating same lesson with same times) |

### schedule handler (5 tests)

- `List_Empty_200` — empty week structure returned
- `Create_OkAsTeacher_201`
- `Create_OverlapsExisting_409`
- `Update_NotMember_403`
- `Delete_NotFound_404`

### notification trigger (4 cases added)

- `Dedup_WithinCooldown_NoSecondNotification` — second alert in 5-min window doesn't fire
- `Dedup_AfterCooldown_FiresAgain`
- `ThresholdBoundary_ExactlyAt_NoFire` — temp == 30 doesn't trigger; > 30 does
- `MissingFCMToken_GracefullyHandled` — when user has no token, broker still publishes

### notification handler (4 tests)

- `List_PaginationClamp_DefaultsTo50` — limit=0 → 50
- `List_AuthzScopedToCaller_DoesNotLeak` — user A can't see user B's notifications
- `MarkRead_NotFound_404`
- `MarkAllRead_Idempotent_204`

### classroom handler (5 tests)

- `Create_AsAdmin_201`
- `Create_AsTeacher_201` (creator becomes member by virtue of `created_by`)
- `List_OnlyShowsOwnedOrMembered_OK`
- `Update_NonOwnerNonAdmin_403`
- `Delete_AsOwner_204_AndGoneAfter` — successful delete returns 204; subsequent GET on the same id returns 404. (DB-level FK cascade is exercised by the migration; handler tests don't need to re-prove it via the in-memory repo.)

## 5. Bumping coverage — math check

Current 29.8% total ≈ 1100 of 3700 statements covered (rough).

New tests will exercise:
- 5 scene handler paths × ~30 statements = 150 stmts
- 6 schedule overlap × shared 50 stmts each = 100 new stmts (high overlap)
- 5 schedule handler × ~30 stmts = 150
- 4 notification handler × ~25 stmts = 100
- 5 classroom handler × ~30 stmts = 150
- 4 notification trigger × shared 60 stmts = 80

Total ~700 net new statements covered. After: (1100 + 700) / 3700 ≈ 48% — comfortably above the 35% gate.

If actual numbers fall short (e.g., heavy code overlap means we only realize 400 net stmts), 41% — still above gate.

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| In-memory repo doesn't faithfully simulate Postgres semantics (cascade, FK, race) | Tests focus on handler logic, not DB semantics. `*test/memrepo.go` already exists and is used by service tests; we're reusing the proven harness. |
| hass build tag split misses a slow test in the default file | Track via the `go test -timeout=10s` heuristic in the plan: any default-build hass test that takes >1s is misplaced. CI will catch it. |
| 35% threshold too strict / too lax | Tune in the same PR after measuring real post-implementation coverage. Threshold is one-line in CI. |
| Handler tests rely on the i18n bundle path resolution from `runtime.Caller` (already used in auth_handler_test.go) | Existing `localesDir(t)` helper works; new tests use the same pattern. |
| `metrics.Reset()` race between parallel tests | New handler tests don't use `t.Parallel()`. Sequential-by-default is fine for our scale. |

## 7. Done criteria

- `go test ./internal/hass/...` runs in **< 5 seconds** (was 55s).
- `go test -tags=slow ./internal/hass/...` runs the original slow tests; both still pass.
- All 5 new handler test files exist (`scene`, `schedule`, `notification`, `classroom`) plus `schedule/service_overlap_test.go`; each test passes under `-race`.
- CI coverage gate is live; threshold = 35%; gate passes.
- Total backend coverage ≥ 40%.
- `staticcheck`, `govulncheck`, `gosec`, `flutter test`, `flutter analyze` all clean.

## 8. Effort estimate

1.5 days end-to-end:
- Half-day: hass slow-test split + verify default-build is fast.
- Half-day: scene + schedule handler + overlap tests.
- Half-day: notification + classroom handler tests + CI gate + final regression.

## 9. After this spec

The next downstream-spec candidate is **WS auth + contract versioning** (Spec 3 from `next-specs.md` — replaces `?access_token=` query param with single-use tickets, tightens WS CheckOrigin, adds `version` field to event schema).
