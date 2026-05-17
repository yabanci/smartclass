-- +goose Up
-- +goose StatementBegin
-- Historical no-op: this migration was originally intended to ensure the
-- expires_at index exists, but 00013_refresh_tokens.sql already creates
-- `refresh_tokens_expires_at_idx` with IF NOT EXISTS. The body below is
-- therefore a duplicate of 00013 and has no effect on any database that ran
-- 00013 first. The migration has been deployed (PR #2) and MUST remain in
-- goose history to preserve version continuity — do NOT delete or renumber it.
-- Kept idempotent via IF NOT EXISTS so applying it to any state is safe.
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Index is retained on Down — dropping it would slow down the purge query on
-- the next Up, and it causes no correctness issues when present.
-- +goose StatementEnd
