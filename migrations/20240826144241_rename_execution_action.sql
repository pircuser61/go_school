-- +goose Up
-- +goose StatementBegin
UPDATE dict_actions SET title = 'Создать поручение' WHERE id = 'new_execution_task';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE dict_actions SET title = 'Создать подзадачу' WHERE id = 'new_execution_task';
-- +goose StatementEnd
