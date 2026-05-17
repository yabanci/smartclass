-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS refresh_tokens (
    jti        UUID PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issued_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ  NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx     ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx  ON refresh_tokens (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd
