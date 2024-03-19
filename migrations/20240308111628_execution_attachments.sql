-- +goose Up
-- +goose StatementBegin
UPDATE dict_actions SET attachments_enabled = true WHERE id = 'execution';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE dict_actions SET attachments_enabled = false WHERE id = 'execution';
-- +goose StatementEnd
