package classroom_test

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
	"smartclass/internal/user"
)

func classroomLocalesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "locales")
}

func newClassroomHandler(t *testing.T, principal middleware.Principal) (chi.Router, *classroom.Service) {
	t.Helper()
	repo := classroomtest.NewMemRepo()
	svc := classroom.NewService(repo)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(classroomLocalesDir(t)))
	h := classroom.NewHandler(svc, validation.New(), bundle)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.WithPrincipalForTest(r.Context(), principal)
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
	router, _ := newClassroomHandler(t, middleware.Principal{UserID: uuid.New(), Role: string(user.RoleAdmin)})
	rec := classroomReq(t, router, http.MethodPost, "/classrooms", map[string]any{"name": "admin-room"})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestClassroomHandler_Create_AsTeacher_201(t *testing.T) {
	router, _ := newClassroomHandler(t, middleware.Principal{UserID: uuid.New(), Role: string(user.RoleTeacher)})
	rec := classroomReq(t, router, http.MethodPost, "/classrooms", map[string]any{"name": "teacher-room"})
	assert.Equal(t, http.StatusCreated, rec.Code,
		"teachers must be able to create classrooms — they're the primary persona for the app")
}

func TestClassroomHandler_List_OnlyShowsOwnedOrMember(t *testing.T) {
	teacherID := uuid.New()
	router, svc := newClassroomHandler(t, middleware.Principal{UserID: teacherID, Role: string(user.RoleTeacher)})

	// Seed a classroom owned by ANOTHER teacher.
	_, err := svc.Create(context.Background(), classroom.CreateInput{
		Name: "other-teachers-room", CreatedBy: uuid.New(),
	})
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
		"a teacher must not see classrooms they neither own nor belong to — without scoping, "+
			"every teacher sees every classroom")
}

func TestClassroomHandler_Update_NonOwnerNonAdmin_403(t *testing.T) {
	otherID := uuid.New()
	router, svc := newClassroomHandler(t, middleware.Principal{UserID: otherID, Role: string(user.RoleTeacher)})

	cls, err := svc.Create(context.Background(), classroom.CreateInput{
		Name: "owned", CreatedBy: uuid.New(),
	})
	require.NoError(t, err)

	name := "renamed"
	rec := classroomReq(t, router, http.MethodPatch, "/classrooms/"+cls.ID.String(),
		map[string]any{"name": &name})
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a teacher who is neither the owner nor an admin must not mutate a classroom")
}

func TestClassroomHandler_Delete_AsOwner_AndGoneAfter(t *testing.T) {
	teacherID := uuid.New()
	router, svc := newClassroomHandler(t, middleware.Principal{UserID: teacherID, Role: string(user.RoleTeacher)})

	cls, err := svc.Create(context.Background(), classroom.CreateInput{
		Name: "to-delete", CreatedBy: teacherID,
	})
	require.NoError(t, err)

	delRec := classroomReq(t, router, http.MethodDelete, "/classrooms/"+cls.ID.String(), nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, delRec.Code, delRec.Body.String())

	getRec := classroomReq(t, router, http.MethodGet, "/classrooms/"+cls.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, getRec.Code,
		"after a successful delete, GET on the same id must 404")
}
