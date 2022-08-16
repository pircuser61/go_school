-- +goose Up
-- +goose StatementBegin
ALTER TABLE pipeliner.versions
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE;
UPDATE pipeliner.versions
SET updated_at = created_at;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeliner.versions
    DROP COLUMN updated_at;
-- +goose StatementEnd
