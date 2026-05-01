package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// DBTracer satisfies pgx.QueryTracer. It records a counter per query result
// and a duration histogram. The op name comes from the context — repo
// functions call WithDBOp(ctx, "users.GetByEmail") before issuing a query.
// Queries without an op annotation fall back to op="unknown" so dashboards
// still show traffic while we progressively annotate repos.
type DBTracer struct{}

func NewDBTracer() *DBTracer { return &DBTracer{} }

type dbOpCtxKey struct{}
type dbStartCtxKey struct{}

// WithDBOp annotates ctx with the op name the next pgx query should report
// under. Repos call this just before pool.Query/QueryRow/Exec.
func WithDBOp(ctx context.Context, op string) context.Context {
	return context.WithValue(ctx, dbOpCtxKey{}, op)
}

func dbOpFrom(ctx context.Context) string {
	if op, ok := ctx.Value(dbOpCtxKey{}).(string); ok && op != "" {
		return op
	}
	return "unknown"
}

func (DBTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, dbStartCtxKey{}, time.Now())
}

func (DBTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	op := dbOpFrom(ctx)
	result := "ok"
	if data.Err != nil {
		result = "err"
	}
	DBQueries.WithLabelValues(op, result).Inc()
	if start, ok := ctx.Value(dbStartCtxKey{}).(time.Time); ok {
		DBDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
	}
}
