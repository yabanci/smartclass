-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS devices (
    id           UUID PRIMARY KEY,
    classroom_id UUID         NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    type         VARCHAR(40)  NOT NULL,
    brand        VARCHAR(40)  NOT NULL,
    driver       VARCHAR(40)  NOT NULL,
    config       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status       VARCHAR(20)  NOT NULL DEFAULT 'unknown',
    online       BOOLEAN      NOT NULL DEFAULT FALSE,
    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS devices_classroom_idx ON devices(classroom_id);
CREATE INDEX IF NOT EXISTS devices_driver_idx ON devices(driver);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS devices;
-- +goose StatementEnd
