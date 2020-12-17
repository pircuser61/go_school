-- +goose Up
-- +goose StatementBegin
ALTER TABLE pipeliner.works
    ADD COLUMN debug  bool  NOT NULL DEFAULT FALSE,
    ADD COLUMN inputs jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeliner.works
    DROP COLUMN debug,
    DROP COLUMN inputs;
-- +goose StatementEnd
