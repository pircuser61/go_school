-- +goose Up
-- +goose StatementBegin
ALTER TABLE version_settings ADD COLUMN notify_process_finished BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE version_settings DROP COLUMN notify_process_finished;
-- +goose StatementEnd
