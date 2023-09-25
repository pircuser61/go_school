-- +goose Up
-- +goose StatementBegin
ALTER TABLE versions
    ADD COLUMN IF NOT EXISTS node_groups jsonb NOT NULL DEFAULT '{}'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE versions
    DROP COLUMN IF  EXISTS node_groups;
-- +goose StatementEnd
