-- +goose Up
-- +goose StatementBegin
INSERT INTO dict_actions VALUES ('new_execution_task', 'Создать подзадачу', true, false, false, 'execution');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM dict_actions WHERE id = 'new_execution_task';
-- +goose StatementEnd
