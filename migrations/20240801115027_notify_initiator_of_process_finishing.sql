-- +goose Up
-- +goose StatementBegin
ALTER TABLE version_settings ADD COLUMN notify_process_finished BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN notify_process_finished SET DEFAULT TRUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE version_settings DROP COLUMN notify_process_finished;

ALTER TABLE IF EXISTS version_settings
    ALTER COLUMN notify_process_finished SET DEFAULT FALSE;
-- +goose StatementEnd
