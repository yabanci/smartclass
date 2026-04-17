package generic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/generic"
)

func TestDriver_Execute_CallsConfiguredEndpoint(t *testing.T) {
	var hitMethod, hitPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitMethod = r.Method
		hitPath = r.URL.Path + "?" + r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"ison": true})
	}))
	defer srv.Close()

	d := generic.New(srv.Client())
	target := devicectl.Target{
		Brand: "shelly",
		Config: map[string]any{
			"baseUrl": srv.URL,
			"commands": map[string]any{
				"ON":  map[string]any{"method": "POST", "path": "/relay/0?turn=on"},
				"OFF": map[string]any{"method": "POST", "path": "/relay/0?turn=off"},
			},
		},
	}

	res, err := d.Execute(context.Background(), target, devicectl.Command{Type: devicectl.CmdOn})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, hitMethod)
	assert.Contains(t, hitPath, "turn=on")
	assert.Equal(t, devicectl.StatusOn, res.Status)
	assert.True(t, res.Online)
}

func TestDriver_Execute_UnsupportedCommand(t *testing.T) {
	d := generic.New(nil)
	_, err := d.Execute(context.Background(), devicectl.Target{
		Config: map[string]any{
			"baseUrl":  "http://x",
			"commands": map[string]any{"ON": map[string]any{"method": "POST", "path": "/on"}},
		},
	}, devicectl.Command{Type: devicectl.CmdOpen})
	require.ErrorIs(t, err, devicectl.ErrUnsupportedCommand)
}

func TestDriver_Execute_InvalidConfig(t *testing.T) {
	d := generic.New(nil)
	_, err := d.Execute(context.Background(), devicectl.Target{Config: map[string]any{}}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrInvalidConfig)
}

func TestDriver_Execute_RemoteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := generic.New(srv.Client())
	_, err := d.Execute(context.Background(), devicectl.Target{
		Config: map[string]any{
			"baseUrl":  srv.URL,
			"commands": map[string]any{"ON": map[string]any{"method": "POST", "path": "/x"}},
		},
	}, devicectl.Command{Type: devicectl.CmdOn})
	require.ErrorIs(t, err, devicectl.ErrUnavailable)
}

func TestDriver_Probe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "off"})
	}))
	defer srv.Close()

	d := generic.New(srv.Client())
	res, err := d.Probe(context.Background(), devicectl.Target{
		Config: map[string]any{
			"baseUrl": srv.URL,
			"status":  map[string]any{"method": "GET", "path": "/status"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, devicectl.StatusOff, res.Status)
}

func TestDriver_SubstitutesValue(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 128)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := generic.New(srv.Client())
	_, err := d.Execute(context.Background(), devicectl.Target{
		Config: map[string]any{
			"baseUrl": srv.URL,
			"commands": map[string]any{
				"SET_VALUE": map[string]any{"method": "POST", "path": "/set", "body": `{"v":{{value}}}`},
			},
		},
	}, devicectl.Command{Type: devicectl.CmdSetValue, Value: 42})
	require.NoError(t, err)
	assert.Equal(t, `{"v":42}`, gotBody)
}
