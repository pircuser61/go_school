-- +goose Up
-- +goose StatementBegin
UPDATE dict_actions SET attachments_enabled = true WHERE id='executor_start_work';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE dict_actions SET attachments_enabled = false WHERE id='executor_start_work';
-- +goose StatementEnd