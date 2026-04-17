-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id             UUID PRIMARY KEY,
    email          VARCHAR(320) NOT NULL UNIQUE,
    password_hash  TEXT         NOT NULL,
    full_name      VARCHAR(160) NOT NULL,
    role           VARCHAR(32)  NOT NULL,
    language       VARCHAR(8)   NOT NULL DEFAULT 'en',
    avatar_url     TEXT         NOT NULL DEFAULT '',
    phone          VARCHAR(32)  NOT NULL DEFAULT '',
    birth_date     DATE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS users_email_lower_idx ON users (lower(email));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
