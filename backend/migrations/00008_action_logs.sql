-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS action_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    UUID,
    entity_type VARCHAR(40) NOT NULL,
    entity_id   UUID,
    action      VARCHAR(40) NOT NULL,
    metadata    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS action_logs_actor_idx ON action_logs(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS action_logs_entity_idx ON action_logs(entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS action_logs_created_idx ON action_logs(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS action_logs;
-- +goose StatementEnd
