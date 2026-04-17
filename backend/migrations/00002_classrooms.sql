-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS classrooms (
    id          UUID PRIMARY KEY,
    name        VARCHAR(120) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    created_by  UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS classrooms_created_by_idx ON classrooms(created_by);

CREATE TABLE IF NOT EXISTS classroom_users (
    classroom_id UUID NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, user_id)
);

CREATE INDEX IF NOT EXISTS classroom_users_user_idx ON classroom_users(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS classroom_users;
DROP TABLE IF EXISTS classrooms;
-- +goose StatementEnd
