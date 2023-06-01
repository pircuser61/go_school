-- +goose Up
-- +goose StatementBegin
INSERT INTO dict_actions(id, title, is_public, comment_enabled, attachments_enabled) VALUES ('form_executor_start_work', 'Взять в работу', true, false, false);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM dict_actions where id = 'form_executor_start_work'
-- +goose StatementEnd
