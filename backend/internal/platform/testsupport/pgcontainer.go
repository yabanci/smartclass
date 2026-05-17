//go:build integration

// Package testsupport provides shared test infrastructure for integration tests
// that require real PostgreSQL. All helpers require Docker to be running.
//
// Usage:
//
//	pool, cleanup := testsupport.StartPostgres(t)
//	defer cleanup()
//	testsupport.ResetDB(t, pool)
package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testDBName = "smartclass_test"
	testDBUser = "sc_test"
	testDBPass = "sc_test_secret"
)

// StartPostgres starts a PostgreSQL testcontainer, runs all goose migrations,
// and returns a ready pool plus a cleanup function. The test is skipped if
// INTEGRATION_TESTS is not set (the build tag is the primary gate; the env var
// allows CI to gate at the runner level independently).
//
// The caller must defer the returned cleanup to terminate the container.
func StartPostgres(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	// INTEGRATION_TESTS=skip lets CI skip the container start even when the
	// binary was compiled with -tags=integration (e.g. during a dry-run step).
	// The build tag is the primary gate; this env var is a secondary override.
	if os.Getenv("INTEGRATION_TESTS") == "skip" {
		t.Skip("INTEGRATION_TESTS=skip; skipping integration test")
	}

	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("testsupport: start postgres container: %v", err)
	}

	cleanup := func() {
		if termErr := ctr.Terminate(ctx); termErr != nil {
			t.Logf("testsupport: terminate container: %v", termErr)
		}
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		t.Fatalf("testsupport: connection string: %v", err)
	}

	if err := runMigrations(dsn); err != nil {
		cleanup()
		t.Fatalf("testsupport: run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		cleanup()
		t.Fatalf("testsupport: open pool: %v", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		cleanup()
		t.Fatalf("testsupport: ping pool: %v", err)
	}

	return pool, func() {
		pool.Close()
		cleanup()
	}
}

// runMigrations applies all goose migrations from the backend/migrations dir.
func runMigrations(dsn string) error {
	// Resolve migrations directory relative to this source file so it works
	// regardless of where `go test` is invoked from.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("testsupport: runtime.Caller failed")
	}
	// thisFile = .../backend/internal/platform/testsupport/pgcontainer.go
	// Three "..": testsupport → platform → internal → backend, then + migrations.
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql.DB for goose: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// appTables lists all application tables in dependency order (dependents before
// their parents) so that TRUNCATE ... CASCADE succeeds cleanly. TRUNCATE is
// wrapped in RESTART IDENTITY to reset auto-increment sequences (BIGSERIAL PKs
// on sensor_readings, action_logs) between tests.
var appTables = []string{
	"refresh_tokens",
	"action_logs",
	"sensor_readings",
	"lessons",
	"scenes",
	"devices",
	"notifications",
	"classroom_users",
	"classrooms",
	"hass_config",
	"device_tokens",
	"users",
}

// ResetDB truncates all application tables and restarts their identity
// sequences. Call this at the start of each sub-test or test function so each
// case starts from a clean slate.
func ResetDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	// Build a single TRUNCATE statement; CASCADE handles FK order automatically.
	// We use the ordered list only to avoid deadlocks under parallel table-level locks;
	// CASCADE still fires for any child rows.
	q := "TRUNCATE TABLE " + joinTables() + " RESTART IDENTITY CASCADE"
	if _, err := pool.Exec(ctx, q); err != nil {
		t.Fatalf("testsupport: ResetDB: %v", err)
	}
}

func joinTables() string {
	out := ""
	for i, tbl := range appTables {
		if i > 0 {
			out += ", "
		}
		out += tbl
	}
	return out
}

// ensure stdlib driver is registered for sql.Open("pgx", ...) used by goose.
var _ = stdlib.GetDefaultDriver
