-- +goose Up
-- +goose StatementBegin
ALTER TABLE IF EXISTS works ADD COLUMN finished_at timestamp with time zone;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE IF EXISTS works DROP COLUMN finished_at;
-- +goose StatementEnd
