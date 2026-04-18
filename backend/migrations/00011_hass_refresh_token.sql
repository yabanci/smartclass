-- +goose Up
-- +goose StatementBegin
ALTER TABLE hass_config
    ADD COLUMN IF NOT EXISTS refresh_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expires_at    TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE hass_config
    DROP COLUMN IF EXISTS refresh_token,
    DROP COLUMN IF EXISTS expires_at;
-- +goose StatementEnd
