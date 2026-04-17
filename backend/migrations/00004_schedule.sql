-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS lessons (
    id           UUID PRIMARY KEY,
    classroom_id UUID         NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    subject      VARCHAR(160) NOT NULL,
    teacher_id   UUID         REFERENCES users(id) ON DELETE SET NULL,
    day_of_week  SMALLINT     NOT NULL CHECK (day_of_week BETWEEN 1 AND 7),
    starts_at    TIME         NOT NULL,
    ends_at      TIME         NOT NULL,
    notes        TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS lessons_classroom_day_idx ON lessons(classroom_id, day_of_week);
CREATE INDEX IF NOT EXISTS lessons_teacher_idx ON lessons(teacher_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS lessons;
-- +goose StatementEnd
