-- +goose Up
-- +goose StatementBegin
ALTER TABLE versions
ADD COLUMN is_hidden boolean IF NOT EXISTS DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE versions
DROP COLUMN IF EXISTS is_hidden;
-- +goose StatementEnd
