-- +goose Up
-- +goose StatementBegin
ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN start_schema DROP DEFAULT;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN start_schema DROP NOT NULL;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN end_schema DROP DEFAULT;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN end_schema DROP NOT NULL;

UPDATE version_settings SET start_schema = NULL, end_schema = NULL;
UPDATE external_systems SET input_schema = NULL, output_schema = NULL, input_mapping = NULL, output_mapping = NULL;

UPDATE versions SET content = (content - 'process_settings') WHERE content -> 'process_settings' IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN start_schema SET DEFAULT '{}'::jsonb;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN start_schema SET NOT NULL;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN end_schema SET DEFAULT '{}'::jsonb;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN end_schema SET NOT NULL;
-- +goose StatementEnd
