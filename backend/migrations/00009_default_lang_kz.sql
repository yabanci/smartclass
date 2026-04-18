-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN language SET DEFAULT 'kz';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN language SET DEFAULT 'en';
-- +goose StatementEnd
