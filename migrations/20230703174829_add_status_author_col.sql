-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
ADD COLUMN IF NOT EXISTS status_author VARCHAR(32) DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
DROP COLUMN IF EXISTS status_author;
-- +goose StatementEnd
