-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
ALTER COLUMN status_author TYPE varchar(256);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
ALTER COLUMN status_author TYPE varchar(32);
-- +goose StatementEnd
