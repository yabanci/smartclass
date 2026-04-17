-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS scenes (
    id           UUID PRIMARY KEY,
    classroom_id UUID         NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    steps        JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS scenes_classroom_idx ON scenes(classroom_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS scenes;
-- +goose StatementEnd
