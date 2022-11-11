-- +goose Up
-- +goose StatementBegin
ALTER TABLE works ADD COLUMN IF NOT EXISTS child_id uuid;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works DROP COLUMN IF EXISTS child_id;
-- +goose StatementEnd
