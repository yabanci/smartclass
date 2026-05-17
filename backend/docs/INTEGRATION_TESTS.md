# Integration Tests

Integration tests exercise real PostgreSQL via [Testcontainers-Go](https://golang.testcontainers.org/).
They catch SQL bugs (broken queries, schema drift, transaction races, FK violations) that mocks cannot.

## Prerequisites

- Docker running locally (`docker info` must succeed).
- Go 1.25+.

## Running locally

From the `backend/` directory:

```sh
go test -tags=integration -race -count=1 ./...
```

The `INTEGRATION_TESTS=skip` env var can skip container startup even when the
binary is compiled with `-tags=integration` (useful for dry-run CI steps):

```sh
INTEGRATION_TESTS=skip go test -tags=integration ./...
```

## Test structure

Each domain repo has a parallel `postgres_integration_test.go` file alongside
its existing unit tests. All integration files carry the `//go:build integration`
build tag so they are invisible to the normal `go test ./...` run.

| File | Cases covered |
|------|---------------|
| `internal/auth/postgres_integration_test.go` | Token issue/use/revoke/replay-detect; idempotency; WHERE clause for used vs live tokens |
| `internal/classroom/postgres_integration_test.go` | CRUD; creator auto-membership; cascade delete to classroom_users; ListForUser scoping; idempotent Assign |
| `internal/device/postgres_integration_test.go` | CRUD; JSONB config round-trip; classroom scoping; UpdateState; cascade delete from classroom |
| `internal/schedule/postgres_integration_test.go` | CRUD; overlap rejection; boundary (adjacent) non-conflict; cross-day non-conflict; UpdateIfNoOverlap self-update; cascade delete |
| `internal/analytics/postgres_integration_test.go` | SensorSeries hour bucket; cross-classroom filter; empty range; invalid bucket; EnergyTotal multi-device sum; DeviceUsage ranking |
| `internal/auditlog/postgres_integration_test.go` | Single/batch insert; empty insert no-op; List filters (ActorID, EntityType, time range); default limit cap; Offset pagination; JSONB metadata round-trip (M-201 guard) |

## Shared helper: `internal/platform/testsupport`

`testsupport.StartPostgres(t)` — boots a `postgres:16-alpine` container, runs
all goose migrations from `backend/migrations/`, and returns a `*pgxpool.Pool`
plus a `cleanup()` function that terminates the container.

`testsupport.ResetDB(t, pool)` — `TRUNCATE ... RESTART IDENTITY CASCADE` on all
application tables in safe dependency order. Call at the start of each sub-test.

## CI

The `.github/workflows/ci.yml` `integration` job runs on `ubuntu-latest` where
Docker is available by default. It runs in parallel with the `mobile` job; the
`docker` image-build job waits for all three (`backend`, `mobile`, `integration`).
