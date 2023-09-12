-- +goose Up
-- +goose StatementBegin
ALTER TABLE external_systems
    ADD COLUMN IF NOT EXISTS allow_run_as_others boolean NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE external_systems
    DROP COLUMN IF EXISTS allow_run_as_others;
-- +goose StatementEnd
