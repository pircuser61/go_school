-- +goose Up
-- +goose StatementBegin
ALTER TABLE versions
ADD COLUMN IF NOT EXISTS is_hidden boolean DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE versions
DROP COLUMN IF EXISTS is_hidden;
-- +goose StatementEnd
