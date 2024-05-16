-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN IF NOT EXISTS checkpoint varchar;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN IF EXISTS checkpoint;
-- +goose StatementEnd
