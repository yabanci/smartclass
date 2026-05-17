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
	"smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
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

// fakeDriver lets us choose ok/err per test without spinning up a real
// HTTP-talking driver. Name() must be a stable string so device.Service can
// look it up via the Factory.
type fakeDriver struct {
	name     string
	applyErr error
}

func (d fakeDriver) Name() string { return d.name }
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
	classroomSvc := classroom.NewService(classroomtest.NewMemRepo())

	factory := devicectl.NewFactory()
	factory.Register(fakeDriver{name: "fake", applyErr: applyErr})
	deviceSvc := device.NewService(devicetest.NewMemRepo(), classroomSvc, factory, nil)

	svc := scene.NewService(scenetest.NewMemRepo(), classroomSvc, deviceSvc, nil)

	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	h := scene.NewHandler(svc, validation.New(), bundle)

	teacherID := uuid.New()
	teacher := classroom.Principal{UserID: teacherID, Role: user.RoleTeacher}
	other := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}

	cls, err := classroomSvc.Create(context.Background(), classroom.CreateInput{
		Name: "test-room", CreatedBy: teacherID,
	})
	require.NoError(t, err)

	dev, err := deviceSvc.Create(context.Background(), teacher, device.CreateInput{
		ClassroomID: cls.ID,
		Name:        "lamp", Type: "light", Brand: "fake", Driver: "fake",
		Config: map[string]any{"baseUrl": "http://x"},
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
			ctx := middleware.WithPrincipalForTest(r.Context(),
				middleware.Principal{UserID: p.UserID, Role: string(p.Role)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sceneReq(t *testing.T, r chi.Router, method, path string, body any) *httptest.ResponseRecorder {
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

func TestSceneHandler_Run_AllOk_200(t *testing.T) {
	h := newSceneHarness(t, nil)
	rec := sceneReq(t, h.router, http.MethodPost, "/scenes/"+h.sceneID.String()+"/run", nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSceneHandler_Run_AnyStepFails_207MultiStatus(t *testing.T) {
	h := newSceneHarness(t, devicectl.ErrUnavailable)
	rec := sceneReq(t, h.router, http.MethodPost, "/scenes/"+h.sceneID.String()+"/run", nil)
	assert.Equal(t, http.StatusMultiStatus, rec.Code,
		"when any step fails, scene.Run returns 207 with per-step results in the body — "+
			"the client must see exactly which step failed, not just a generic 5xx")

	// The body must include a steps array so the client can render which
	// specific commands didn't apply.
	var resp struct {
		Data struct {
			Steps []struct {
				Success bool   `json:"success"`
				Error   string `json:"error,omitempty"`
			} `json:"steps"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Steps, 2, "both step results must be reported even when both failed")
	for i, s := range resp.Data.Steps {
		assert.False(t, s.Success, "step %d must report success=false when its driver returned an error", i)
		assert.NotEmpty(t, s.Error, "step %d must include the error message in its result", i)
	}
}

func TestSceneHandler_Run_NotFound_404(t *testing.T) {
	h := newSceneHarness(t, nil)
	rec := sceneReq(t, h.router, http.MethodPost, "/scenes/"+uuid.NewString()+"/run", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSceneHandler_Run_NonMember_403(t *testing.T) {
	h := newSceneHarness(t, nil)
	bundle := i18n.NewBundle(i18n.EN)
	require.NoError(t, bundle.LoadDir(localesDir(t)))
	otherRouter := chi.NewRouter()
	otherRouter.Use(injectScenePrincipal(h.other))
	handler := scene.NewHandler(h.svc, validation.New(), bundle)
	otherRouter.Route("/scenes", handler.Routes)

	rec := sceneReq(t, otherRouter, http.MethodPost, "/scenes/"+h.sceneID.String()+"/run", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a non-member must not be able to run scenes — that would let any teacher activate any classroom's devices")
}

func TestSceneHandler_List_OnlyOwnClassroom_OK(t *testing.T) {
	h := newSceneHarness(t, nil)
	rec := sceneReq(t, h.router, http.MethodGet, "/classrooms/"+h.classroomID.String()+"/scenes", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}
