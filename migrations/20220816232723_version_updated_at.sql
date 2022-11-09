-- +goose Up
-- +goose StatementBegin
ALTER TABLE versions
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE;
UPDATE versions
SET updated_at = created_at;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE versions
    DROP COLUMN updated_at;
-- +goose StatementEnd
