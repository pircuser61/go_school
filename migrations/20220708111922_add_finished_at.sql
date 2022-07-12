-- +goose Up
-- +goose StatementBegin
ALTER TABLE IF EXISTS pipeliner.works ADD COLUMN finished_at timestamp with time zone;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE IF EXISTS pipeliner.works DROP COLUMN finished_at;
-- +goose StatementEnd
