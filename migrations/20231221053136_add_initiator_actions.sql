-- +goose Up
-- +goose StatementBegin
insert into dict_actions(
    id,
    title,
    is_public,
    comment_enabled,
    attachments_enabled
)
values
('reply_execution_info', 'Ответить', true, true, true),
('reply_approver_info', 'Ответить', true, true, true),
('repeat_app', 'Повторить', true, true, false),
('create_new_app', 'Вернуть в работу ', true, true, false);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM dict_actions
WHERE id IN(
    'reply_execution_info',
    'reply_approver_info',
    'repeat_app',
    'create_new_app'
);
-- +goose StatementEnd
