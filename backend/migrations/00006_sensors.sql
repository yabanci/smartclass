-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sensor_readings (
    id          BIGSERIAL PRIMARY KEY,
    device_id   UUID        NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    metric      VARCHAR(40) NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    unit        VARCHAR(20) NOT NULL DEFAULT '',
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw         JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS sensor_readings_device_metric_time_idx
    ON sensor_readings(device_id, metric, recorded_at DESC);
CREATE INDEX IF NOT EXISTS sensor_readings_time_idx
    ON sensor_readings(recorded_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sensor_readings;
-- +goose StatementEnd
