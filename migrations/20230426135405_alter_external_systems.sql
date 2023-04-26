-- +goose Up
-- +goose StatementBegin
ALTER TABLE external_systems
    ADD COLUMN IF NOT EXISTS microservice_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ending_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sending_method text NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE external_systems
    DROP COLUMN IF EXISTS microservice_id,
    DROP COLUMN IF EXISTS ending_url,
    DROP COLUMN IF EXISTS sending_method;
-- +goose StatementEnd
