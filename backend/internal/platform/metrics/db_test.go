package metrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/platform/metrics"
)

func TestDBTracer_RecordsOkQuery(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	ctx := metrics.WithDBOp(context.Background(), "users.GetByEmail")
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT * FROM users WHERE email=$1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	require.Equal(t, 1.0, testutil.ToFloat64(metrics.DBQueries.WithLabelValues("users.GetByEmail", "ok")))
	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.DBDuration), 1)
}

func TestDBTracer_RecordsErrQuery(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	ctx := metrics.WithDBOp(context.Background(), "users.Insert")
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("constraint violation")})

	got := testutil.ToFloat64(metrics.DBQueries.WithLabelValues("users.Insert", "err"))
	assert.Equal(t, 1.0, got, "queries returning a non-nil Err must be counted under result=err")
}

func TestDBTracer_FallsBackToUnknownWhenOpMissing(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	// No WithDBOp — bare context. The tracer must not panic and must label
	// with "unknown" so we can still see *something* in dashboards while
	// repos are progressively annotated.
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	assert.Equal(t, 1.0, testutil.ToFloat64(metrics.DBQueries.WithLabelValues("unknown", "ok")),
		"queries without an op annotation must fall back to op=unknown so dashboards still show traffic")
}

func TestDBTracer_DurationStartsAtTraceQueryStart(t *testing.T) {
	metrics.Reset()
	tr := metrics.NewDBTracer()

	ctx := metrics.WithDBOp(context.Background(), "slow.op")
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{})
	time.Sleep(10 * time.Millisecond)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.DBDuration), 1)
}
