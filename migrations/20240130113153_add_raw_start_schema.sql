-- +goose Up
-- +goose StatementBegin
ALTER TABLE version_settings
ADD COLUMN raw_start_schema jsonb;
UPDATE version_settings SET raw_start_schema = start_schema;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE version_settings
DROP COLUMN IF EXISTS raw_start_schema;
-- +goose StatementEnd
