-- +goose Up
-- +goose StatementBegin
ALTER TABLE versions
ADD is_hidden boolean;

ALTER TABLE versions ALTER COLUMN is_hidden
SET DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE versions
DROP COLUMN is_hidden;
-- +goose StatementEnd
