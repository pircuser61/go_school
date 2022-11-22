-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN run_context JSONB NOT NULL DEFAULT '{}'::JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN run_context;
-- +goose StatementEnd
