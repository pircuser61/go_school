-- +goose Up
-- +goose StatementBegin
ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN input_schema DROP DEFAULT;

ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN input_schema DROP NOT NULL;

ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN output_schema DROP DEFAULT;

ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN output_schema DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN input_schema SET DEFAULT '{}'::jsonb;

ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN input_schema SET NOT NULL;

ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN output_schema SET DEFAULT '{}'::jsonb;

ALTER TABLE IF EXISTS external_systems
    ALTER COLUMN output_schema SET NOT NULL;
-- +goose StatementEnd
