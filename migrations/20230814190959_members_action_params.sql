-- +goose Up
-- +goose StatementBegin
ALTER TABLE pipeliner.public.members
    ADD COLUMN params JSONB DEFAULT '{}'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeliner.public.members
    DROP COLUMN params;
-- +goose StatementEnd
