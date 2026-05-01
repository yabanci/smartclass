package generic_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/generic"
	"smartclass/internal/platform/metrics"
)

func TestGenericDriver_IncrementsMetricOnExecute(t *testing.T) {
	metrics.Reset()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := generic.New(server.Client())
	target := devicectl.Target{
		Config: map[string]any{
			"baseUrl": server.URL,
			"commands": map[string]any{
				"ON": map[string]any{"method": "POST", "path": "/relay/0?turn=on"},
			},
		},
	}
	_, err := d.Execute(context.Background(), target, devicectl.Command{Type: devicectl.CmdOn})
	require.NoError(t, err)

	got := testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("generic_http", "ON", "ok"))
	require.Equal(t, 1.0, got,
		"a successful Execute call must increment the driver counter labeled (generic_http, ON, ok)")
}
