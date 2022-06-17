-- +goose Up
-- +goose StatementBegin
ALTER TABLE pipeliner.works ADD COLUMN human_status TEXT NOT NULL DEFAULT 'new';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeliner.works DROP COLUMN human_status;
-- +goose StatementEnd
