package metrics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestTrackDriver_OkPath(t *testing.T) {
	metrics.Reset()
	err := metrics.TrackDriver(context.Background(), "generic_http", "ON", func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("generic_http", "ON", "ok")))
	assert.GreaterOrEqual(t, testutil.CollectAndCount(metrics.DriverDuration), 1)
}

func TestTrackDriver_ErrPath_LabelsAsErr(t *testing.T) {
	metrics.Reset()
	wantErr := errors.New("boom")
	err := metrics.TrackDriver(context.Background(), "homeassistant", "OFF", func(_ context.Context) error {
		return wantErr
	})
	require.ErrorIs(t, err, wantErr,
		"TrackDriver must return the inner function's error verbatim — instrumentation never swallows errors")
	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.DriverCalls.WithLabelValues("homeassistant", "OFF", "err")))
}

func TestTrackHass_Counts(t *testing.T) {
	metrics.Reset()
	_ = metrics.TrackHass(context.Background(), "ListEntities", func(_ context.Context) error { return nil })
	_ = metrics.TrackHass(context.Background(), "AbortFlow", func(_ context.Context) error { return errors.New("404") })

	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.HassCalls.WithLabelValues("ListEntities", "ok")))
	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.HassCalls.WithLabelValues("AbortFlow", "err")))
}
