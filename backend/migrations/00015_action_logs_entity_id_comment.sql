-- +goose Up
-- +goose StatementBegin
-- entity_id intentionally has NO foreign key to devices.id.
-- Analytics retention requires orphaned rows to remain after a device is
-- deleted (audit history must survive the referenced object). A FK would
-- cascade or restrict deletes and destroy that history.
COMMENT ON COLUMN action_logs.entity_id IS
    'UUID of the entity the action targeted. '
    'No FK to devices.id by design — orphaned rows are retained for '
    'analytics and audit after the device is deleted.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
COMMENT ON COLUMN action_logs.entity_id IS NULL;
-- +goose StatementEnd
