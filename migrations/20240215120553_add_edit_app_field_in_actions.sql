-- +goose Up
-- +goose StatementBegin
INSERT INTO dict_actions values ('edit_app', 'Отредактировать', true, false, false, '');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE from dict_actions where id = 'edit_app';
-- +goose StatementEnd
