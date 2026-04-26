package homeassistant_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/homeassistant"
)

func fakeHA(t *testing.T) (*httptest.Server, *struct {
	method, path, auth string
	body               map[string]any
}) {
	t.Helper()
	rec := &struct {
		method, path, auth string
		body               map[string]any
	}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.auth = r.Header.Get("Authorization")
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &rec.body)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/services/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
		case strings.HasPrefix(r.URL.Path, "/api/states/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state":      "on",
				"attributes": map[string]any{"friendly_name": "Lamp"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func TestExecute_Switch_On(t *testing.T) {
	srv, rec := fakeHA(t)
	d := homeassistant.New(srv.Client(), nil)
	target := devicectl.Target{Config: map[string]any{
		"baseUrl":  srv.URL,
		"token":    "xyz",
		"entityId": "switch.classroom_lamp",
	}}
	res, err := d.Execute(context.Background(), target, devicectl.Command{Type: devicectl.CmdOn})
	require.NoError(t, err)
	assert.Equal(t, devicectl.StatusOn, res.Status)
	assert.True(t, res.Online)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api/services/switch/turn_on", rec.path)
	assert.Equal(t, "Bearer xyz", rec.auth)
	assert.Equal(t, "switch.classroom_lamp", rec.body["entity_id"])
}

func TestExecute_Cover_Open(t *testing.T) {
	srv, rec := fakeHA(t)
	d := homeassistant.New(srv.Client(), nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl":  srv.URL,
		"token":    "xyz",
		"entityId": "cover.curtain",
	}}, devicectl.Command{Type: devicectl.CmdOpen})
	require.NoError(t, err)
	assert.Equal(t, "/api/services/cover/open_cover", rec.path)
}

func TestExecute_Light_SetBrightness(t *testing.T) {
	srv, rec := fakeHA(t)
	d := homeassistant.New(srv.Client(), nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl":  srv.URL,
		"token":    "xyz",
		"entityId": "light.main",
	}}, devicectl.Command{Type: devicectl.CmdSetValue, Value: 75})
	require.NoError(t, err)
	assert.Equal(t, "/api/services/light/turn_on", rec.path)
	assert.EqualValues(t, 75, rec.body["brightness_pct"])
}

func TestExecute_InvalidConfig(t *testing.T) {
	d := homeassistant.New(nil, nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl": "http://x", "token": "", "entityId": "switch.x",
	}}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrInvalidConfig)
}

func TestExecute_EntityIdMustHaveDomain(t *testing.T) {
	d := homeassistant.New(nil, nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl": "http://x", "token": "t", "entityId": "no_domain",
	}}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrInvalidConfig)
}

func TestExecute_Unsupported_SetValue_ForSwitch(t *testing.T) {
	srv, _ := fakeHA(t)
	d := homeassistant.New(srv.Client(), nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl": srv.URL, "token": "t", "entityId": "switch.x",
	}}, devicectl.Command{Type: devicectl.CmdSetValue, Value: 50})
	require.ErrorIs(t, err, devicectl.ErrUnsupportedCommand)
}

func TestExecute_5xxPropagatesUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	d := homeassistant.New(srv.Client(), nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl": srv.URL, "token": "t", "entityId": "switch.x",
	}}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrUnavailable)
}

func TestProbe(t *testing.T) {
	srv, _ := fakeHA(t)
	d := homeassistant.New(srv.Client(), nil)
	res, err := d.Probe(context.Background(), devicectl.Target{Config: map[string]any{
		"baseUrl": srv.URL, "token": "t", "entityId": "switch.x",
	}})
	require.NoError(t, err)
	assert.Equal(t, devicectl.StatusOn, res.Status)
	assert.True(t, res.Online)
}
