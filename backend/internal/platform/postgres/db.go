package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type DB struct {
	Pool *pgxpool.Pool
	dsn  string
}

func Connect(ctx context.Context, dsn string, tracer pgx.QueryTracer) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
	}

	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	return &DB{Pool: pool, dsn: dsn}, nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// Ready satisfies server.ReadinessChecker by pinging the pool. Returns
// promptly on context cancellation so the readiness probe stays responsive
// even when the DB is wedged.
func (db *DB) Ready(ctx context.Context) error {
	if db.Pool == nil {
		return fmt.Errorf("postgres: pool not initialized")
	}
	if err := db.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", err)
	}
	return nil
}

func (db *DB) Migrate(dir string) error {
	sqlDB, err := sql.Open("pgx", db.dsn)
	if err != nil {
		return fmt.Errorf("postgres: open for migrate: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("postgres: set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, dir); err != nil {
		return fmt.Errorf("postgres: goose up: %w", err)
	}
	return nil
}

var _ = stdlib.GetDefaultDriver
