-- +goose Up
-- +goose StatementBegin
-- users_email_lower_idx was a functional index on lower(email) created in
-- 00001_init.sql. The lookup query in user/postgres.go uses `email = $1`
-- (equality on the stored column, which is always lower-cased at write time
-- via normalizeEmail), so the planner never selects this index — it falls back
-- to a sequential scan or uses the primary-key / btree index on email directly.
-- Dropping it removes dead maintenance overhead on every INSERT/UPDATE.
DROP INDEX IF EXISTS users_email_lower_idx;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS users_email_lower_idx ON users(lower(email));
-- +goose StatementEnd
