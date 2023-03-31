-- +goose Up
-- +goose StatementBegin
ALTER TABLE version_settings ADD COLUMN resubmission_period int NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE version_settings DROP COLUMN resubmission_period;
-- +goose StatementEnd
