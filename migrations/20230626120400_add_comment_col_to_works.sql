-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
ADD COLUMN IF NOT EXISTS status_comment VARCHAR(256) DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
DROP COLUMN IF EXISTS status_comment;
-- +goose StatementEnd
