-- +goose Up
-- +goose StatementBegin
ALTER TABLE  variable_storage ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage
DROP COLUMN IF EXISTS is_active;
-- +goose StatementEnd