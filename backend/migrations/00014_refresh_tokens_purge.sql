-- +goose Up
-- +goose StatementBegin
-- Ensure the expires_at index exists (may already exist from 00013).
-- The startup purge goroutine uses this index for the periodic DELETE.
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Index is retained on Down — dropping it would slow down the purge query on
-- the next Up, and it causes no correctness issues when present.
-- +goose StatementEnd
