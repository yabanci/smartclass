-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS device_tokens (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        TEXT        NOT NULL,
    platform     TEXT        NOT NULL CHECK (platform IN ('android', 'ios', 'web')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS device_tokens_user_token_uidx
    ON device_tokens(user_id, token);

CREATE INDEX IF NOT EXISTS device_tokens_user_idx
    ON device_tokens(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS device_tokens;
-- +goose StatementEnd
