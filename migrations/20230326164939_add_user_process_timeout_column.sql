-- +goose Up
-- +goose StatementBegin
ALTER TABLE version_settings ADD COLUMN user_process_timeout int NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE version_settings DROP COLUMN user_process_timeout;
-- +goose StatementEnd
