-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS fcm_token TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS fcm_token;
