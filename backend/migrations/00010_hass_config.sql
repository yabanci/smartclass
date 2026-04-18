-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS hass_config (
    id          SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    base_url    TEXT        NOT NULL,
    token       TEXT        NOT NULL,
    onboarded   BOOLEAN     NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS hass_config;
-- +goose StatementEnd
