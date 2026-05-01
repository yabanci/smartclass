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
	"smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
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
	other       classroom.Principal
}

// newScheduleHarness builds a router with both `/schedule` and
// `/classrooms/{id}/schedule` mounted, an injected principal (defaults to the
// classroom-owning teacher), one seeded classroom, and a service backed by
// in-memory repos.
func newScheduleHarness(t *testing.T, principal classroom.Principal) *scheduleHarness {
	t.Helper()
	classroomSvc := classroom.NewService(classroomtest.NewMemRepo())
	svc := schedule.NewService(scheduletest.NewMemRepo(), classroomSvc)

	cls, err := classroomSvc.Create(context.Background(), classroom.CreateInput{
		Name:      "test-room",
		CreatedBy: principal.UserID,
	})
	require.NoError(t, err)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	h := schedule.NewHandler(svc, validation.New(), bundle)

	r := chi.NewRouter()
	r.Use(injectSchedulePrincipal(principal))
	r.Route("/schedule", h.Routes)
	r.Route("/classrooms/{id}/schedule", h.ClassroomRoutes)

	return &scheduleHarness{
		router:      r,
		svc:         svc,
		classroomID: cls.ID,
		teacher:     principal,
		other:       classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher},
	}
}

func injectSchedulePrincipal(p classroom.Principal) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.WithPrincipalForTest(r.Context(),
				middleware.Principal{UserID: p.UserID, Role: string(p.Role)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func doSchedReq(t *testing.T, r chi.Router, method, path string, body any) *httptest.ResponseRecorder {
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

func TestHandler_ListWeek_Empty_200(t *testing.T) {
	teacher := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
	h := newScheduleHarness(t, teacher)

	rec := doSchedReq(t, h.router, http.MethodGet,
		"/classrooms/"+h.classroomID.String()+"/schedule", nil)

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHandler_CreateLesson_OkAsTeacher_201(t *testing.T) {
	teacher := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
	h := newScheduleHarness(t, teacher)

	rec := doSchedReq(t, h.router, http.MethodPost, "/schedule",
		map[string]any{
			"classroomId": h.classroomID.String(),
			"subject":     "math",
			"dayOfWeek":   int(schedule.Monday),
			"startsAt":    "09:00",
			"endsAt":      "10:00",
		})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestHandler_CreateLesson_Overlap_409(t *testing.T) {
	teacher := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
	h := newScheduleHarness(t, teacher)

	body := map[string]any{
		"classroomId": h.classroomID.String(),
		"subject":     "math",
		"dayOfWeek":   int(schedule.Monday),
		"startsAt":    "09:00",
		"endsAt":      "10:00",
	}
	first := doSchedReq(t, h.router, http.MethodPost, "/schedule", body)
	require.Equal(t, http.StatusCreated, first.Code, "first create must succeed")

	rec := doSchedReq(t, h.router, http.MethodPost, "/schedule", body)
	assert.Equal(t, http.StatusConflict, rec.Code,
		"second create with the exact same window must surface as 409 Conflict — that's the contract for overlap")
}

func TestHandler_DeleteLesson_NotFound_404(t *testing.T) {
	teacher := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
	h := newScheduleHarness(t, teacher)

	rec := doSchedReq(t, h.router, http.MethodDelete, "/schedule/"+uuid.NewString(), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"deleting a non-existent lesson must 404 — the handler must distinguish 'not in DB' from 'DB broken'")
}

func TestHandler_UpdateLesson_NonOwner_403(t *testing.T) {
	teacher := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
	h := newScheduleHarness(t, teacher)

	// Create a lesson as the owner.
	created := doSchedReq(t, h.router, http.MethodPost, "/schedule",
		map[string]any{
			"classroomId": h.classroomID.String(),
			"subject":     "x",
			"dayOfWeek":   int(schedule.Monday),
			"startsAt":    "09:00",
			"endsAt":      "10:00",
		})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &resp))

	// Re-mount the same handler under a router that injects the OTHER
	// principal — they aren't a member of the classroom and must be rejected.
	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	otherRouter := chi.NewRouter()
	otherRouter.Use(injectSchedulePrincipal(h.other))
	handler := schedule.NewHandler(h.svc, validation.New(), bundle)
	otherRouter.Route("/schedule", handler.Routes)

	subject := "renamed"
	rec := doSchedReq(t, otherRouter, http.MethodPatch, "/schedule/"+resp.Data.ID,
		map[string]any{"subject": &subject})
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a teacher who is not a member of the classroom must not be able to update its schedule")
}
