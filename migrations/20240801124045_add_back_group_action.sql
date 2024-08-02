-- +goose Up
-- +goose StatementBegin
INSERT INTO dict_actions values ('back_to_group', 'Вернуть в очередь', true, false, false, 'execution');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE from dict_actions where id = 'back_to_group';
-- +goose StatementEnd
