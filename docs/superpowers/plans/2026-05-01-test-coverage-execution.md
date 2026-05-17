# Test Coverage Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Critical-path test coverage: hass slow-test isolation, handler tests for scene/schedule/notification/classroom, schedule overlap correctness, CI coverage gate at 35%.

**Architecture:** Reuse the proven pattern from `backend/internal/auth/handler_test.go` (mustJSON helper, chi-router-per-test, in-memory repos from each package's `*test/memrepo.go`). Slow `hass` tests get a `//go:build slow` build tag so default `go test` skips them and CI runs both fast + slow tracks. CI gets a `go tool cover -func | awk` step that fails the build below 35% total coverage.

**Tech Stack:** Go 1.25 · chi v5.2.5 · testify · existing in-memory repos · `prometheus/client_golang/prometheus/testutil` for metric counter checks where handlers increment them.

**Source spec:** `docs/superpowers/specs/2026-05-01-test-coverage-design.md`

---

## File map

```
backend/
├── .github/workflows/ci.yml                              ← +1 step "Coverage gate" + +1 step "hass slow tests"
├── internal/hass/
│   ├── service_test.go                                   ← keep ONLY fast tests
│   └── service_slow_test.go (NEW)                        ← `//go:build slow` — backoff/poll-bound tests
├── internal/scene/handler_test.go (NEW)                  ← 5 tests
├── internal/schedule/
│   ├── handler_test.go (NEW)                             ← 5 tests
│   └── service_overlap_test.go (NEW)                     ← 6 table-driven cases
├── internal/notification/
│   ├── handler_test.go (NEW)                             ← 4 tests
│   └── trigger_test.go                                   ← +4 cases (extend existing file)
└── internal/classroom/handler_test.go (NEW)              ← 5 tests
```

**Per-task discipline.** Each task is its own commit. Run `go test -race ./<changed>/...` after each task; final task runs full regression (vet + staticcheck + govulncheck + gosec + race + flutter).

**Working directory.** `backend/`-prefixed paths are relative to `/Users/arsenozhetov/Projects/pet/smartclass`. Run go commands from `backend/`.

---

## Task 1: Diagnose + split hass slow tests behind `//go:build slow`

**Files:**
- Read: `backend/internal/hass/service_test.go` (find which tests are slow)
- Modify: `backend/internal/hass/service_test.go` (remove the slow tests)
- Create: `backend/internal/hass/service_slow_test.go` (move slow tests here with build tag)

- [ ] **Step 1: Identify slow tests by name**

```bash
cd backend && go test -v ./internal/hass/... 2>&1 | grep -E "PASS|FAIL" | awk '{
  test=$3; dur=$NF;
  gsub(/[()]/,"",dur);
  print dur, test
}' | sort -rn | head -10
```

Expected: top of list shows tests at 7.5s+ (BootstrapWithRetry path).

- [ ] **Step 2: Identify the package + imports of the slow file**

Read `backend/internal/hass/service_test.go`:

```bash
head -30 backend/internal/hass/service_test.go
```

Note the package name (likely `hass` or `hass_test`) and the imports.

- [ ] **Step 3: Cut the slow tests into a new file with build tag**

Identify the slow test function names from Step 1 (typically `TestBootstrap_OnboardsOnce`, `TestListEntities_FiltersToSupportedDomains`, `TestFlowProxy_StartAndStep`, `TestCredentials_SerializesConcurrentRefresh`, `TestCurrentToken_ReturnsBootstrappedToken`, `TestCurrentToken_RefreshesExpiredToken`, `TestStatus_ReflectsConfiguredState` — verify with Step 1).

Move them to a new file `backend/internal/hass/service_slow_test.go`. The file MUST start with the build tag on the very first line:

```go
//go:build slow

package <same package name as service_test.go>

import (
	// same imports as service_test.go for the moved tests
)

// All tests in this file exercise the BootstrapWithRetry / token-refresh /
// flow-proxy paths that include real backoff or polling delays. Default
// `go test` skips them; CI runs them in a dedicated step:
//   go test -tags=slow ./internal/hass/...

// ... moved test functions ...
```

Remove the same functions from `service_test.go`. If a moved test references a helper defined in `service_test.go`, copy that helper into the new file too (or extract it into a non-build-tagged helper file `service_helper_test.go`).

- [ ] **Step 4: Verify default-build is fast**

```bash
cd backend && time go test ./internal/hass/...
```

Expected: PASS in **< 5 seconds** (was 55s).

- [ ] **Step 5: Verify slow tests still run with the tag**

```bash
cd backend && go test -tags=slow ./internal/hass/...
```

Expected: PASS in ~50-60s (the original slow tests + the new fast ones together).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/hass/
git -c commit.gpgsign=false commit -m "test(hass): split slow tests behind //go:build slow tag

Default 'go test ./...' now runs hass tests in <5s (was 55s). Slow
tests exercising BootstrapWithRetry backoff and token refresh polling
still run with: go test -tags=slow ./internal/hass/...

Closes audit finding F-023."
```

---

## Task 2: schedule overlap correctness — table-driven service test

**Files:**
- Create: `backend/internal/schedule/service_overlap_test.go`

- [ ] **Step 1: Read existing schedule service test for setup pattern**

```bash
head -40 backend/internal/schedule/service_test.go
```

Note: how it constructs `*Service` (memrepo + classroom service), how it sets up a classroom + a teacher principal. Reuse the same construction.

- [ ] **Step 2: Write the failing test file**

Create `backend/internal/schedule/service_overlap_test.go`:

```go
package schedule_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/schedule"
	"smartclass/internal/schedule/scheduletest"
	"smartclass/internal/user"
)

// TestService_Overlap_TableDriven exercises the overlap-detection contract
// at the boundaries that have historically hidden bugs: half-open intervals,
// same-minute boundaries, edit-of-self, end-equal-start rejection, different
// days. Every case sets up the same anchor lesson (Monday 09:00-10:00) in
// the same classroom, then attempts to add or update a second lesson and
// asserts on the result code.
func TestService_Overlap_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		day       schedule.DayOfWeek
		startsAt  schedule.TimeOfDay
		endsAt    schedule.TimeOfDay
		wantErr   error
		errReason string
	}{
		{
			name:      "edge_touch_after_anchor_is_ok_half_open",
			day:       schedule.Monday,
			startsAt:  schedule.MakeTimeOfDay(10, 0),
			endsAt:    schedule.MakeTimeOfDay(11, 0),
			wantErr:   nil,
			errReason: "anchor ends at 10:00 (exclusive); a new lesson starting AT 10:00 must NOT conflict with half-open semantics",
		},
		{
			name:      "overlap_in_middle_is_conflict",
			day:       schedule.Monday,
			startsAt:  schedule.MakeTimeOfDay(9, 30),
			endsAt:    schedule.MakeTimeOfDay(10, 30),
			wantErr:   schedule.ErrOverlap,
			errReason: "any minute inside [09:00, 10:00) must be rejected as overlapping the anchor",
		},
		{
			name:      "edge_touch_before_anchor_is_ok",
			day:       schedule.Monday,
			startsAt:  schedule.MakeTimeOfDay(8, 0),
			endsAt:    schedule.MakeTimeOfDay(9, 0),
			wantErr:   nil,
			errReason: "anchor starts at 09:00; a new lesson ending AT 09:00 is back-to-back, not overlapping",
		},
		{
			name:      "end_equals_start_is_invalid_time",
			day:       schedule.Monday,
			startsAt:  schedule.MakeTimeOfDay(11, 0),
			endsAt:    schedule.MakeTimeOfDay(11, 0),
			wantErr:   schedule.ErrInvalidTime,
			errReason: "zero-length lessons must be rejected at validation, before overlap is even consulted",
		},
		{
			name:      "different_day_no_conflict",
			day:       schedule.Tuesday,
			startsAt:  schedule.MakeTimeOfDay(9, 0),
			endsAt:    schedule.MakeTimeOfDay(10, 0),
			wantErr:   nil,
			errReason: "the same wall-clock window on a different weekday must not be a conflict — overlap is per-day",
		},
		{
			name:      "fully_inside_anchor_is_conflict",
			day:       schedule.Monday,
			startsAt:  schedule.MakeTimeOfDay(9, 15),
			endsAt:    schedule.MakeTimeOfDay(9, 45),
			wantErr:   schedule.ErrOverlap,
			errReason: "a window fully contained inside the anchor must be a conflict",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, principal, classroomID := newScheduleSvcWithAnchor(t)

			_, err := svc.Create(context.Background(), principal, schedule.CreateInput{
				ClassroomID: classroomID,
				Subject:     "second lesson",
				DayOfWeek:   tc.day,
				StartsAt:    tc.startsAt,
				EndsAt:      tc.endsAt,
			})

			if tc.wantErr == nil {
				require.NoError(t, err, tc.errReason)
			} else {
				require.ErrorIs(t, err, tc.wantErr, tc.errReason)
			}
		})
	}
}

// TestService_Overlap_EditSelf_NoChange covers the "I'm just renaming my own
// lesson" flow: updating a lesson with its own (start, end, day) values must
// not be reported as overlapping itself.
func TestService_Overlap_EditSelf_NoChange(t *testing.T) {
	svc, principal, classroomID := newScheduleSvcWithAnchor(t)

	// The anchor is already at Mon 09:00-10:00. Find its id, then issue an
	// update with the same times (just rename the subject).
	week, err := svc.Week(context.Background(), principal, classroomID)
	require.NoError(t, err)
	require.Len(t, week[schedule.Monday], 1, "exactly one anchor lesson seeded")
	anchor := week[schedule.Monday][0]

	newSubject := "renamed"
	updated, err := svc.Update(context.Background(), principal, anchor.ID, schedule.UpdateInput{
		Subject: &newSubject,
	})
	assert.NoError(t, err,
		"updating a lesson without changing day/start/end must succeed — overlap-with-self is the canonical false-positive bug")
	assert.Equal(t, "renamed", updated.Subject)
}

// newScheduleSvcWithAnchor wires a schedule service with a memory backend,
// seeds one classroom owned by a teacher, places one Mon 09:00-10:00 anchor
// lesson, and returns (service, teacher principal, classroom id).
func newScheduleSvcWithAnchor(t *testing.T) (*schedule.Service, classroom.Principal, uuid.UUID) {
	t.Helper()
	classroomRepo := classroomtest.NewMemRepo()
	classroomSvc := classroom.NewService(classroomRepo)
	repo := scheduletest.NewMemRepo()
	svc := schedule.NewService(repo, classroomSvc)

	teacherID := uuid.New()
	principal := classroom.Principal{UserID: teacherID, Role: user.RoleTeacher}

	created, err := classroomSvc.Create(context.Background(), classroom.CreateInput{
		Name:      "test-room",
		CreatedBy: teacherID,
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), principal, schedule.CreateInput{
		ClassroomID: created.ID,
		Subject:     "anchor",
		DayOfWeek:   schedule.Monday,
		StartsAt:    schedule.MakeTimeOfDay(9, 0),
		EndsAt:      schedule.MakeTimeOfDay(10, 0),
	})
	require.NoError(t, err)

	return svc, principal, created.ID
}
```

- [ ] **Step 3: Verify needed helpers / constants exist; if not, the test names them**

```bash
grep -n "MakeTimeOfDay\|TimeOfDay\b\|DayOfWeek\b" backend/internal/schedule/lesson.go | head -10
```

If `MakeTimeOfDay(h, m int) TimeOfDay` doesn't exist, the test file will define a helper. Inspect `lesson.go` to see how `TimeOfDay` is constructed today (likely just `TimeOfDay(h*60 + m)`); if so, add this small helper at the top of `service_overlap_test.go`:

```go
// MakeTimeOfDay is a readable shorthand for the constructor pattern
// `TimeOfDay(h*60+m)` used throughout the schedule package.
func makeTOD(h, m int) schedule.TimeOfDay { return schedule.TimeOfDay(h*60 + m) }
```

…and replace `schedule.MakeTimeOfDay(...)` calls in the table with `makeTOD(...)`.

- [ ] **Step 4: Run tests to verify**

```bash
cd backend && go test -race -count=1 ./internal/schedule/... -run 'TestService_Overlap_'
```

Expected: PASS for all 7 cases (6 in the table + the edit-self test).

If a case fails, that's a real bug — investigate before adjusting the test. The "edge_touch" cases assume half-open `[start, end)` semantics; if the implementation uses closed intervals, the failing test surfaces the incorrect behavior.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/schedule/service_overlap_test.go
git -c commit.gpgsign=false commit -m "test(schedule): table-driven overlap correctness (7 boundary cases)"
```

---

## Task 3: schedule handler tests

**Files:**
- Create: `backend/internal/schedule/handler_test.go`

- [ ] **Step 1: Read the existing handler to know the routes**

```bash
grep -n "func.*Routes\|Get\|Post\|Put\|Delete\|Patch" backend/internal/schedule/handler.go | head -20
```

Note the route paths under `/schedule/...` and `/classrooms/{id}/schedule/...`.

- [ ] **Step 2: Write the failing test file**

Create `backend/internal/schedule/handler_test.go`:

```go
package schedule_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/platform/i18n"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/validation"
	"smartclass/internal/schedule"
	"smartclass/internal/schedule/scheduletest"
	"smartclass/internal/user"
)

func localesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

type scheduleHarness struct {
	router      chi.Router
	svc         *schedule.Service
	classroomID uuid.UUID
	teacher     classroom.Principal
	otherUser   classroom.Principal
}

func newScheduleHarness(t *testing.T) *scheduleHarness {
	t.Helper()
	classroomRepo := classroomtest.NewMemRepo()
	classroomSvc := classroom.NewService(classroomRepo)
	repo := scheduletest.NewMemRepo()
	svc := schedule.NewService(repo, classroomSvc)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	h := schedule.NewHandler(svc, validation.New(), bundle)

	teacherID := uuid.New()
	teacher := classroom.Principal{UserID: teacherID, Role: user.RoleTeacher}
	other := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}

	cls, err := classroomSvc.Create(context.Background(), classroom.CreateInput{
		Name:      "test-room",
		CreatedBy: teacherID,
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(injectPrincipalMiddleware(teacher)) // default: teacher principal
	r.Route("/classrooms/{id}/schedule", h.ClassroomRoutes)
	r.Route("/schedule", h.Routes)

	return &scheduleHarness{
		router: r, svc: svc, classroomID: cls.ID, teacher: teacher, otherUser: other,
	}
}

// injectPrincipalMiddleware lets handler tests bypass the real Authn
// middleware (which would need a JWT) by stuffing a principal directly into
// the request context. Production handlers read mw.PrincipalFrom(ctx); this
// shim populates the same key.
func injectPrincipalMiddleware(p classroom.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.WithPrincipalForTest(r.Context(), mw.Principal{UserID: p.UserID, Role: string(p.Role)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

func doScheduleRequest(t *testing.T, h *scheduleHarness, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = mustJSON(t, body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestHandler_ListWeek_Empty_200(t *testing.T) {
	h := newScheduleHarness(t)
	rec := doScheduleRequest(t, h, http.MethodGet,
		"/classrooms/"+h.classroomID.String()+"/schedule", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Body shape is the week map; the response wraps it under "data". We
	// don't deserialize the whole map — verifying status is enough for the
	// pagination-shaped happy path.
}

func TestHandler_CreateLesson_OkAsTeacher_201(t *testing.T) {
	h := newScheduleHarness(t)
	rec := doScheduleRequest(t, h, http.MethodPost,
		"/classrooms/"+h.classroomID.String()+"/schedule",
		map[string]any{
			"subject":   "math",
			"dayOfWeek": int(schedule.Monday),
			"startsAt":  9 * 60,
			"endsAt":    10 * 60,
		})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestHandler_CreateLesson_Overlap_409(t *testing.T) {
	h := newScheduleHarness(t)
	body := map[string]any{
		"subject":   "math",
		"dayOfWeek": int(schedule.Monday),
		"startsAt":  9 * 60,
		"endsAt":    10 * 60,
	}
	first := doScheduleRequest(t, h, http.MethodPost, "/classrooms/"+h.classroomID.String()+"/schedule", body)
	require.Equal(t, http.StatusCreated, first.Code, "first create must succeed")

	rec := doScheduleRequest(t, h, http.MethodPost, "/classrooms/"+h.classroomID.String()+"/schedule", body)
	assert.Equal(t, http.StatusConflict, rec.Code,
		"second create with the exact same window must surface as 409 Conflict — that's the contract for overlap")
}

func TestHandler_DeleteLesson_NotFound_404(t *testing.T) {
	h := newScheduleHarness(t)
	rec := doScheduleRequest(t, h, http.MethodDelete,
		"/schedule/"+uuid.NewString(), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"deleting a non-existent lesson must return 404, not 500 — the handler must distinguish 'not in DB' from 'DB broken'")
}

func TestHandler_UpdateLesson_NonOwner_403(t *testing.T) {
	h := newScheduleHarness(t)

	// Create as the (default-injected) teacher.
	created := doScheduleRequest(t, h, http.MethodPost, "/classrooms/"+h.classroomID.String()+"/schedule",
		map[string]any{"subject": "x", "dayOfWeek": int(schedule.Monday), "startsAt": 9 * 60, "endsAt": 10 * 60})
	require.Equal(t, http.StatusCreated, created.Code)
	var resp struct {
		Data struct {
			ID string `json:"id"`
		}
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &resp))

	// Switch the router to inject the OTHER user as principal — they're not
	// a member of the classroom and must be rejected.
	h.router = chi.NewRouter()
	h.router.Use(injectPrincipalMiddleware(h.otherUser))
	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	handler := schedule.NewHandler(h.svc, validation.New(), bundle)
	h.router.Route("/schedule", handler.Routes)

	subject := "renamed"
	rec := doScheduleRequest(t, h, http.MethodPatch, "/schedule/"+resp.Data.ID,
		map[string]any{"subject": &subject})
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a teacher who is not a member of the classroom must not be able to update its schedule")
}
```

- [ ] **Step 3: If `mw.WithPrincipalForTest` does not exist, add it**

```bash
grep -n "WithPrincipalForTest" backend/internal/platform/httpx/middleware/auth.go
```

If absent, append to `backend/internal/platform/httpx/middleware/auth.go`:

```go
// WithPrincipalForTest installs a Principal directly in the context, used by
// handler tests that don't go through the real Authn middleware. Production
// code never calls this — use Authn.
func WithPrincipalForTest(ctx context.Context, p Principal) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, p.UserID)
	ctx = context.WithValue(ctx, ctxKeyRole, p.Role)
	return ctx
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/schedule/... -run 'TestHandler_'
```

Expected: PASS for all 5 handler tests.

If a test fails, the failure surfaces a real handler-behavior gap or a routing mismatch. Fix the handler OR adjust the test path/payload as needed.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/schedule/handler_test.go backend/internal/platform/httpx/middleware/auth.go
git -c commit.gpgsign=false commit -m "test(schedule): handler tests (list/create/overlap/update-authz/delete-404)"
```

---

## Task 4: scene handler tests

**Files:**
- Create: `backend/internal/scene/handler_test.go`

- [ ] **Step 1: Read scene handler routes**

```bash
grep -n "func.*Routes\|Get\|Post\|Put\|Delete\|Patch\|run" backend/internal/scene/handler.go
```

Note the Run endpoint path.

- [ ] **Step 2: Write the test file**

Create `backend/internal/scene/handler_test.go`:

```go
package scene_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/device"
	"smartclass/internal/device/devicetest"
	"smartclass/internal/devicectl"
	"smartclass/internal/platform/i18n"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/validation"
	"smartclass/internal/scene"
	"smartclass/internal/scene/scenetest"
	"smartclass/internal/user"
)

func localesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

// fakeFactory is a devicectl.Factory whose driver is configurable per-test:
// `applyErr` controls whether the driver succeeds or returns an error on
// every Apply, letting Run tests cover ok/partial/all-fail in one harness.
type fakeFactory struct {
	applyErr error
}

func (f fakeFactory) Driver(_ string) (devicectl.Driver, error) {
	return fakeDriver{applyErr: f.applyErr}, nil
}

func (f fakeFactory) IsRegistered(_ string) bool { return true }

type fakeDriver struct{ applyErr error }

func (d fakeDriver) Name() string { return "fake" }
func (d fakeDriver) Execute(_ context.Context, _ devicectl.Target, _ devicectl.Command) (devicectl.Result, error) {
	return devicectl.Result{Online: true}, d.applyErr
}
func (d fakeDriver) Probe(_ context.Context, _ devicectl.Target) (devicectl.Result, error) {
	return devicectl.Result{}, nil
}

type sceneHarness struct {
	router      chi.Router
	svc         *scene.Service
	classroomID uuid.UUID
	deviceID    uuid.UUID
	sceneID     uuid.UUID
	teacher     classroom.Principal
	other       classroom.Principal
}

func newSceneHarness(t *testing.T, applyErr error) *sceneHarness {
	t.Helper()
	classroomRepo := classroomtest.NewMemRepo()
	classroomSvc := classroom.NewService(classroomRepo)
	deviceRepo := devicetest.NewMemRepo()
	deviceSvc := device.NewService(deviceRepo, classroomSvc, fakeFactory{applyErr: applyErr}, nil)
	sceneRepo := scenetest.NewMemRepo()
	svc := scene.NewService(sceneRepo, classroomSvc, deviceSvc, nil)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	h := scene.NewHandler(svc, validation.New(), bundle)

	teacherID := uuid.New()
	teacher := classroom.Principal{UserID: teacherID, Role: user.RoleTeacher}
	other := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}

	cls, err := classroomSvc.Create(context.Background(), classroom.CreateInput{
		Name:      "test-room",
		CreatedBy: teacherID,
	})
	require.NoError(t, err)

	dev, err := deviceSvc.Create(context.Background(), teacher, device.CreateInput{
		ClassroomID: cls.ID,
		Name:        "lamp",
		Type:        "light",
		Brand:       "fake",
		Driver:      "fake",
		Config:      map[string]any{"baseUrl": "http://x"},
	})
	require.NoError(t, err)

	sc, err := svc.Create(context.Background(), teacher, scene.CreateInput{
		ClassroomID: cls.ID,
		Name:        "test-scene",
		Steps: []scene.Step{
			{DeviceID: dev.ID, Command: "ON"},
			{DeviceID: dev.ID, Command: "OFF"},
		},
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(injectScenePrincipal(teacher))
	r.Route("/scenes", h.Routes)
	r.Route("/classrooms/{id}/scenes", h.ClassroomRoutes)

	return &sceneHarness{
		router: r, svc: svc, classroomID: cls.ID, deviceID: dev.ID, sceneID: sc.ID,
		teacher: teacher, other: other,
	}
}

func injectScenePrincipal(p classroom.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.WithPrincipalForTest(r.Context(), mw.Principal{UserID: p.UserID, Role: string(p.Role)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sceneReq(t *testing.T, h *sceneHarness, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestSceneHandler_Run_AllOk_200(t *testing.T) {
	h := newSceneHarness(t, nil) // every step succeeds
	rec := sceneReq(t, h, http.MethodPost, "/scenes/"+h.sceneID.String()+"/run", nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSceneHandler_Run_AllStepsFail_502(t *testing.T) {
	h := newSceneHarness(t, devicectl.ErrUnavailable)
	rec := sceneReq(t, h, http.MethodPost, "/scenes/"+h.sceneID.String()+"/run", nil)
	assert.Equal(t, http.StatusBadGateway, rec.Code,
		"when every step fails, scene.ErrStepFailed maps to 502 — that's the contract for upstream-driver failure")
}

func TestSceneHandler_Run_NotFound_404(t *testing.T) {
	h := newSceneHarness(t, nil)
	rec := sceneReq(t, h, http.MethodPost, "/scenes/"+uuid.NewString()+"/run", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSceneHandler_Run_NonMember_403(t *testing.T) {
	h := newSceneHarness(t, nil)
	// Switch principal to a user who isn't a member of the classroom.
	h.router = chi.NewRouter()
	h.router.Use(injectScenePrincipal(h.other))
	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	handler := scene.NewHandler(h.svc, validation.New(), bundle)
	h.router.Route("/scenes", handler.Routes)

	rec := sceneReq(t, h, http.MethodPost, "/scenes/"+h.sceneID.String()+"/run", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a non-member must not be able to run scenes — that would let any teacher activate any classroom's devices")
}

func TestSceneHandler_List_OnlyOwnClassroom_OK(t *testing.T) {
	h := newSceneHarness(t, nil)
	rec := sceneReq(t, h, http.MethodGet, "/classrooms/"+h.classroomID.String()+"/scenes", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}
```

- [ ] **Step 3: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/scene/... -run 'TestSceneHandler_'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/scene/handler_test.go
git -c commit.gpgsign=false commit -m "test(scene): handler tests (run-ok/all-fail/404/403/list)"
```

---

## Task 5: notification trigger expansions

**Files:**
- Modify: `backend/internal/notification/trigger_test.go`

- [ ] **Step 1: Read existing trigger_test for the harness**

```bash
head -60 backend/internal/notification/trigger_test.go
```

Note how it constructs the engine + service.

- [ ] **Step 2: Append four cases**

Open `backend/internal/notification/trigger_test.go`. At the bottom of the file (after the existing tests), append:

```go
func TestEngine_Dedup_WithinCooldown_NoSecondNotification(t *testing.T) {
	engine, svc, classroomID, deviceID := newEngineHarness(t)

	engine.OnSensorReading(context.Background(), classroomID, deviceID, "temperature", 35.0, "C")
	engine.OnSensorReading(context.Background(), classroomID, deviceID, "temperature", 36.0, "C")

	list, err := svc.List(context.Background(), defaultMember(classroomID), false, 50, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1,
		"a second high-temperature reading inside the cooldown window must NOT fire — "+
			"otherwise a flapping sensor spams the user")
}

func TestEngine_ThresholdBoundary_ExactlyAt_NoFire(t *testing.T) {
	engine, svc, classroomID, deviceID := newEngineHarness(t)

	// DefaultRules.TemperatureHighC == 30. Strictly greater triggers; equal must not.
	engine.OnSensorReading(context.Background(), classroomID, deviceID, "temperature", 30.0, "C")
	list, err := svc.List(context.Background(), defaultMember(classroomID), false, 50, 0)
	require.NoError(t, err)
	assert.Empty(t, list,
		"a reading exactly at the threshold must NOT fire — the rule is `>` not `>=`, otherwise every borderline reading alerts")
}

func TestEngine_ThresholdBoundary_JustAbove_Fires(t *testing.T) {
	engine, svc, classroomID, deviceID := newEngineHarness(t)

	engine.OnSensorReading(context.Background(), classroomID, deviceID, "temperature", 30.001, "C")
	list, err := svc.List(context.Background(), defaultMember(classroomID), false, 50, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1, "the smallest possible reading above the threshold must fire — that's the alert's job")
}

func TestEngine_DeviceOffline_Fires(t *testing.T) {
	engine, svc, classroomID, deviceID := newEngineHarness(t)

	engine.OnDeviceStateChange(context.Background(), classroomID, deviceID, "lamp", false, "lamp")

	list, err := svc.List(context.Background(), defaultMember(classroomID), false, 50, 0)
	require.NoError(t, err)
	require.Len(t, list, 1, "a device transitioning to offline must alert the classroom members")
	assert.Equal(t, notification.TypeWarning, list[0].Type)
}
```

If `newEngineHarness` and `defaultMember` don't already exist in `trigger_test.go`, add them at the bottom too:

```go
func newEngineHarness(t *testing.T) (*notification.Engine, *notification.Service, uuid.UUID, uuid.UUID) {
	t.Helper()
	classroomID := uuid.New()
	memberID := uuid.New()
	deviceID := uuid.New()

	repo := notificationtest.NewMemRepo()
	members := membersStub{classroomID: classroomID, member: memberID}
	svc := notification.NewService(repo, members, nil)
	engine := notification.NewEngine(svc, notification.DefaultRules())

	return engine, svc, classroomID, deviceID
}

// membersStub satisfies notification.MemberLookup with a single classroom→member mapping.
type membersStub struct {
	classroomID uuid.UUID
	member      uuid.UUID
}

func (m membersStub) Members(_ context.Context, classroomID uuid.UUID) ([]uuid.UUID, error) {
	if classroomID == m.classroomID {
		return []uuid.UUID{m.member}, nil
	}
	return nil, nil
}

// defaultMember returns the member-id from membersStub for use as the
// `userID` arg to Service.List in the assertions above. The harness above
// always uses one fixed member id, so we expose it through a global so
// individual tests can assert without rebuilding the harness.
var harnessMemberID uuid.UUID

func defaultMember(_ uuid.UUID) uuid.UUID {
	if harnessMemberID == uuid.Nil {
		harnessMemberID = uuid.New()
	}
	return harnessMemberID
}
```

(Note: if existing tests already provide a similar harness with a different name, prefer extending it instead of duplicating. Skim the current file before adding.)

- [ ] **Step 3: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/notification/...
```

Expected: PASS — including the new 4 cases.

If a "fires/doesn't fire" assertion fails, that's a real semantics bug in `trigger.go` (probably `>` vs `>=` confusion). Fix the trigger before adjusting the test.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/notification/trigger_test.go
git -c commit.gpgsign=false commit -m "test(notification): trigger dedup + threshold boundary + offline cases"
```

---

## Task 6: notification handler tests

**Files:**
- Create: `backend/internal/notification/handler_test.go`

- [ ] **Step 1: Write the test file**

Create `backend/internal/notification/handler_test.go`:

```go
package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/notification"
	"smartclass/internal/notification/notificationtest"
	"smartclass/internal/platform/i18n"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/user"
)

func notificationLocalesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

type notifHarness struct {
	router chi.Router
	svc    *notification.Service
	userID uuid.UUID
}

func newNotifHarness(t *testing.T) *notifHarness {
	t.Helper()
	repo := notificationtest.NewMemRepo()
	svc := notification.NewService(repo, &fixedMembers{}, nil)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(notificationLocalesDir(t)))
	h := notification.NewHandler(svc, bundle)

	uid := uuid.New()
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.WithPrincipalForTest(r.Context(), mw.Principal{UserID: uid, Role: string(user.RoleTeacher)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/notifications", h.Routes)

	return &notifHarness{router: r, svc: svc, userID: uid}
}

type fixedMembers struct{}

func (fixedMembers) Members(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func TestNotifHandler_List_PaginationClampsZeroLimit(t *testing.T) {
	h := newNotifHarness(t)
	// Seed 100 notifications for the principal user.
	for i := 0; i < 100; i++ {
		_, err := h.svc.CreateForUser(context.Background(), notification.Input{
			UserID: h.userID, Type: notification.TypeInfo,
			Title: "t", Message: "m",
		})
		require.NoError(t, err)
	}

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications?limit=0", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.LessOrEqual(t, len(resp.Data), 50,
		"limit=0 must clamp to the default page size (50), not return all 100 — otherwise a malicious client can DoS by requesting 'no limit'")
}

func TestNotifHandler_List_AuthzScopedToCaller(t *testing.T) {
	h := newNotifHarness(t)

	// Insert a notification for someone OTHER than the principal user.
	other := uuid.New()
	_, err := h.svc.CreateForUser(context.Background(), notification.Input{
		UserID: other, Type: notification.TypeInfo, Title: "secret", Message: "private",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data,
		"the principal user has no notifications of their own, so the list must be empty — "+
			"the OTHER user's notification must NOT leak. Without scoping, every user sees every user's alerts")
}

func TestNotifHandler_MarkRead_NotFound_404(t *testing.T) {
	h := newNotifHarness(t)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notifications/"+uuid.NewString()+"/read", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"marking a non-existent notification as read must 404 — the handler must not 500 or 200-with-body")
}

func TestNotifHandler_MarkAllRead_Idempotent(t *testing.T) {
	h := newNotifHarness(t)
	// Seed 3 notifications for the user.
	for i := 0; i < 3; i++ {
		_, err := h.svc.CreateForUser(context.Background(), notification.Input{
			UserID: h.userID, Type: notification.TypeInfo, Title: "t", Message: "m",
		})
		require.NoError(t, err)
	}

	first := httptest.NewRecorder()
	h.router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil))
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, first.Code,
		"first MarkAllRead returns 200/204 — accept either as long as it's success")

	second := httptest.NewRecorder()
	h.router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil))
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, second.Code,
		"calling MarkAllRead twice must be idempotent — re-marking already-read items must not 500")
}
```

- [ ] **Step 2: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/notification/... -run TestNotifHandler_
```

Expected: PASS for all 4. If route paths differ in the actual handler (e.g., `/notifications/read-all` may be a different path), inspect `handler.go` and adjust.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/notification/handler_test.go
git -c commit.gpgsign=false commit -m "test(notification): handler tests (pagination clamp/authz/MarkRead 404/MarkAllRead idempotent)"
```

---

## Task 7: classroom handler tests

**Files:**
- Create: `backend/internal/classroom/handler_test.go`

- [ ] **Step 1: Read classroom handler routes**

```bash
grep -n "func.*Routes\|Get\|Post\|Put\|Delete\|Patch" backend/internal/classroom/handler.go
```

- [ ] **Step 2: Write the test file**

Create `backend/internal/classroom/handler_test.go`:

```go
package classroom_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/platform/i18n"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/validation"
	"smartclass/internal/user"
)

func classroomLocalesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

func newClassroomHandler(t *testing.T, principal mw.Principal) (chi.Router, *classroom.Service) {
	t.Helper()
	repo := classroomtest.NewMemRepo()
	svc := classroom.NewService(repo)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(classroomLocalesDir(t)))
	h := classroom.NewHandler(svc, validation.New(), bundle)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := mw.WithPrincipalForTest(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/classrooms", h.Routes)
	return r, svc
}

func classroomReq(t *testing.T, r chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestClassroomHandler_Create_AsAdmin_201(t *testing.T) {
	router, _ := newClassroomHandler(t, mw.Principal{UserID: uuid.New(), Role: string(user.RoleAdmin)})
	rec := classroomReq(t, router, http.MethodPost, "/classrooms", map[string]any{"name": "admin-room"})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestClassroomHandler_Create_AsTeacher_201(t *testing.T) {
	router, _ := newClassroomHandler(t, mw.Principal{UserID: uuid.New(), Role: string(user.RoleTeacher)})
	rec := classroomReq(t, router, http.MethodPost, "/classrooms", map[string]any{"name": "teacher-room"})
	assert.Equal(t, http.StatusCreated, rec.Code,
		"teachers must be able to create classrooms — they're the primary persona for the app")
}

func TestClassroomHandler_List_OnlyShowsOwnedOrMember(t *testing.T) {
	teacherID := uuid.New()
	router, svc := newClassroomHandler(t, mw.Principal{UserID: teacherID, Role: string(user.RoleTeacher)})

	// Seed a classroom owned by ANOTHER teacher.
	_, err := svc.Create(t.Context(), classroom.CreateInput{Name: "other-teachers-room", CreatedBy: uuid.New()})
	require.NoError(t, err)

	rec := classroomReq(t, router, http.MethodGet, "/classrooms", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data,
		"a teacher must not see classrooms they neither own nor belong to — without scoping, every teacher sees every classroom")
}

func TestClassroomHandler_Update_NonOwnerNonAdmin_403(t *testing.T) {
	otherID := uuid.New()
	router, svc := newClassroomHandler(t, mw.Principal{UserID: otherID, Role: string(user.RoleTeacher)})

	// Seed a classroom owned by someone else.
	cls, err := svc.Create(t.Context(), classroom.CreateInput{Name: "owned", CreatedBy: uuid.New()})
	require.NoError(t, err)

	name := "renamed"
	rec := classroomReq(t, router, http.MethodPatch, "/classrooms/"+cls.ID.String(), map[string]any{"name": &name})
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a teacher who is neither the owner nor an admin must not be able to mutate a classroom — that's the authz contract")
}

func TestClassroomHandler_Delete_AsOwner_AndGoneAfter(t *testing.T) {
	teacherID := uuid.New()
	router, svc := newClassroomHandler(t, mw.Principal{UserID: teacherID, Role: string(user.RoleTeacher)})

	cls, err := svc.Create(t.Context(), classroom.CreateInput{Name: "to-delete", CreatedBy: teacherID})
	require.NoError(t, err)

	delRec := classroomReq(t, router, http.MethodDelete, "/classrooms/"+cls.ID.String(), nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, delRec.Code, delRec.Body.String())

	getRec := classroomReq(t, router, http.MethodGet, "/classrooms/"+cls.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, getRec.Code,
		"after a successful delete, the classroom must be gone — GET on its id must 404")
}
```

- [ ] **Step 3: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/classroom/... -run TestClassroomHandler_
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/classroom/handler_test.go
git -c commit.gpgsign=false commit -m "test(classroom): handler tests (admin/teacher create, list authz, update 403, delete 204)"
```

---

## Task 8: CI coverage gate

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Inspect current backend job**

```bash
grep -n "Coverage\|coverage" .github/workflows/ci.yml
```

Note where `coverage.out` is generated (the existing step `Unit tests (race + coverage)`).

- [ ] **Step 2: Add the gate step**

Open `.github/workflows/ci.yml`. Find the existing "Coverage summary" step (or "Unit tests" if no summary step exists). Insert a new step after coverage is produced:

```yaml
      - name: Coverage gate
        run: |
          pct=$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%","",$3); print int($3)}')
          echo "Total coverage: $pct%"
          if [ -z "$pct" ]; then
            echo "::error::could not parse coverage from coverage.out"
            exit 1
          fi
          if [ "$pct" -lt 35 ]; then
            echo "::error::Total coverage $pct% is below threshold 35%"
            exit 1
          fi
```

The threshold (`35`) is inline. To raise it later, edit one number.

- [ ] **Step 3: Add a separate slow-tests step**

Below the existing backend `Unit tests` step, add:

```yaml
      - name: hass slow tests (build tag)
        run: go test -tags=slow -race -count=1 ./internal/hass/...
```

This runs the slow hass tests on every CI run but in a separate, clearly-named step so a failure is easy to attribute. Locally, default `go test ./...` skips them (Task 1 split).

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git -c commit.gpgsign=false commit -m "ci: coverage gate at 35% + dedicated slow-tag hass test step"
```

---

## Task 9: Final regression sweep

**Files:** No changes — verification only.

- [ ] **Step 1: All Go checks under -race**

```bash
export PATH=$PATH:$(go env GOPATH)/bin
cd backend
go vet ./... && echo "vet OK"
staticcheck ./... && echo "staticcheck OK"
govulncheck ./... 2>&1 | tail -3
gosec -quiet ./... 2>&1 | tail -7
go test -race -count=1 ./... 2>&1 | grep -E "FAIL|ok\s" | tail -25
```

Expected: each tool reports zero issues; all packages PASS under `-race`. The hass package finishes in **< 5 seconds** (was 55s).

- [ ] **Step 2: hass slow tests still pass**

```bash
cd backend && go test -tags=slow -race -count=1 ./internal/hass/...
```

Expected: PASS in ~50-60s.

- [ ] **Step 3: Coverage**

```bash
cd backend && go test -coverprofile=/tmp/cov.out ./... > /dev/null 2>&1
go tool cover -func=/tmp/cov.out | tail -1
rm /tmp/cov.out
```

Expected: total ≥ **40%** (was 29.8% before this spec).

- [ ] **Step 4: Mobile regression**

```bash
cd /Users/arsenozhetov/Projects/pet/smartclass
(cd mobile && flutter analyze && flutter test --reporter=compact 2>&1 | tail -3)
```

Expected: clean analyze; 59/59 tests pass.

- [ ] **Step 5: Final commit if any tweaks were needed**

```bash
git status
# If clean, no commit needed.
# If tweaks made:
git add .
git -c commit.gpgsign=false commit -m "chore(test): regression fixes from final run"
```

- [ ] **Step 6: Print summary to user**

Output: hass runtime before/after (55s → <5s); total coverage before/after (29.8% → 40%+); CI gate live at 35%; ~24 new tests across 5 packages. Reference audit findings closed by this work: F-006 (handler/service coverage), F-023 (hass slow tests).

---

## Self-Review

**Spec coverage check (against `2026-05-01-test-coverage-design.md`):**
- §2 hass slow-test isolation → Task 1 ✓
- §2 schedule overlap correctness (6 cases + edit-self) → Task 2 ✓
- §2 schedule handler tests (5) → Task 3 ✓
- §2 scene handler tests (5) → Task 4 ✓
- §2 notification trigger expansions (4) → Task 5 ✓
- §2 notification handler tests (4) → Task 6 ✓
- §2 classroom handler tests (5) → Task 7 ✓
- §2 CI coverage gate at 35% → Task 8 ✓
- §7 Done criteria (hass < 5s, total ≥ 40%, all linters clean) → Task 9 ✓

**Placeholder scan:** No "TBD"/"TODO"/"fill in details" inside task steps. The CI gate step has the literal threshold `35`. ✓

**Type/name consistency:**
- `mw.WithPrincipalForTest` — Task 3 declares (or skips if exists), Tasks 4, 6, 7 consume.
- `localesDir(t)` — Tasks 3, 4, 6, 7 each define a local copy in their package's test file (avoids cross-package test imports).
- `mustJSON`/`doScheduleRequest`/`sceneReq`/`classroomReq` — each test file owns its request helpers (no shared test util).
- `classroom.Principal` vs `mw.Principal` — Tasks 3, 4 use `classroom.Principal` for service calls and `mw.Principal` for the request-context shim. Conversion is `string(user.RoleX)`. Consistent.
- `metrics.Reset()` — not strictly needed in handler tests since handlers only inc counters, never read; if metrics state needs clean slate per test, add `metrics.Reset()` at the top.

Plan ready.
