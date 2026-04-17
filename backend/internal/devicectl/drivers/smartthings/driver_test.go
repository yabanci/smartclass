package smartthings_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/smartthings"
)

type capture struct {
	method, path, auth string
	body               map[string]any
}

func fakeST(t *testing.T, status map[string]any) (*httptest.Server, *capture) {
	t.Helper()
	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.auth = r.Header.Get("Authorization")
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &rec.body)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path != "":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []any{map[string]any{"id": "1", "status": "ACCEPTED"}},
			})
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(status)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func TestExecute_SwitchOn(t *testing.T) {
	srv, rec := fakeST(t, nil)
	d := smartthings.New(srv.Client(), smartthings.WithBaseURL(srv.URL))

	target := devicectl.Target{Config: map[string]any{
		"token":    "pat-xyz",
		"deviceId": "dev-1",
	}}
	res, err := d.Execute(context.Background(), target, devicectl.Command{Type: devicectl.CmdOn})
	require.NoError(t, err)
	assert.Equal(t, devicectl.StatusOn, res.Status)
	assert.True(t, res.Online)

	assert.Equal(t, "POST", rec.method)
	assert.Equal(t, "/devices/dev-1/commands", rec.path)
	assert.Equal(t, "Bearer pat-xyz", rec.auth)

	cmds := rec.body["commands"].([]any)
	require.Len(t, cmds, 1)
	cmd := cmds[0].(map[string]any)
	assert.Equal(t, "main", cmd["component"])
	assert.Equal(t, "switch", cmd["capability"])
	assert.Equal(t, "on", cmd["command"])
}

func TestExecute_OpenWindowShade(t *testing.T) {
	srv, rec := fakeST(t, nil)
	d := smartthings.New(srv.Client(), smartthings.WithBaseURL(srv.URL))
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"token": "t", "deviceId": "d",
	}}, devicectl.Command{Type: devicectl.CmdOpen})
	require.NoError(t, err)

	cmd := rec.body["commands"].([]any)[0].(map[string]any)
	assert.Equal(t, "windowShade", cmd["capability"])
	assert.Equal(t, "open", cmd["command"])
}

func TestExecute_SetLevel(t *testing.T) {
	srv, rec := fakeST(t, nil)
	d := smartthings.New(srv.Client(), smartthings.WithBaseURL(srv.URL))
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"token": "t", "deviceId": "d",
	}}, devicectl.Command{Type: devicectl.CmdSetValue, Value: 75})
	require.NoError(t, err)

	cmd := rec.body["commands"].([]any)[0].(map[string]any)
	assert.Equal(t, "switchLevel", cmd["capability"])
	assert.Equal(t, "setLevel", cmd["command"])
	args := cmd["arguments"].([]any)
	assert.EqualValues(t, 75, args[0])
}

func TestExecute_CapabilityOverride_Lock(t *testing.T) {
	srv, rec := fakeST(t, nil)
	d := smartthings.New(srv.Client(), smartthings.WithBaseURL(srv.URL))
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"token": "t", "deviceId": "d", "capability": "lock",
	}}, devicectl.Command{Type: devicectl.CmdOpen})
	require.NoError(t, err)
	cmd := rec.body["commands"].([]any)[0].(map[string]any)
	assert.Equal(t, "lock", cmd["capability"])
	assert.Equal(t, "unlock", cmd["command"])
}

func TestExecute_MissingToken(t *testing.T) {
	d := smartthings.New(nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"deviceId": "x",
	}}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrInvalidConfig)
}

func TestExecute_CloudError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	d := smartthings.New(srv.Client(), smartthings.WithBaseURL(srv.URL))
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{
		"token": "t", "deviceId": "d",
	}}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrUnavailable)
}

func TestProbe_Switch(t *testing.T) {
	status := map[string]any{
		"components": map[string]any{
			"main": map[string]any{
				"switch": map[string]any{
					"switch": map[string]any{"value": "on"},
				},
			},
		},
	}
	srv, _ := fakeST(t, status)
	d := smartthings.New(srv.Client(), smartthings.WithBaseURL(srv.URL))
	res, err := d.Probe(context.Background(), devicectl.Target{Config: map[string]any{
		"token": "t", "deviceId": "d",
	}})
	require.NoError(t, err)
	assert.Equal(t, devicectl.StatusOn, res.Status)
}
