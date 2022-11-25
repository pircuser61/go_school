-- +goose Up
-- +goose StatementBegin
create table if not exists dict_actions(
    id character varying not null,
    title character varying,
    is_public bool default true not null,
    comment_enabled bool default false not null,
    attachments_enabled bool default false not null,

    constraint dict_actions_pkey primary key (id)
);

insert into dict_actions(id,
                         title,
                         is_public,
                         comment_enabled,
                         attachments_enabled)
values
-- executors
('execution', 'Решить', true, true, false),
('executor_start_work', 'Взять в работу', true, false, false),
('change_executor', 'Переназначить', true, true, false),
('request_execution_info', 'Запросить информацию', true, true, true),
-- form
('fill_form', '', true, false, false), -- "fill_form"
-- approvers

('request_add_info', 'Запросить информацию', true, true, true),
('send_edit_app', 'Вернуть на доработку', true, true, true),
('approve', 'Согласовать', true, true, false),
('reject', 'Отклонить', true, true, false),
('viewed', 'Ознакомлен', true, true, false),
('informed', 'Проинформирован', true, true, false),
('sign', 'Подписать', true, true, false),
('affirmate', 'Утвердить', true, true, false),
-- misc
('cancel_app', 'Отозвать', true, true, false),

-- non public actions
('add_approvers', 'Добавить согласующего', false, true, false),
('additional_approvement', '', false, false, false),
('additional_reject', '', false, false, false);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists dict_actions;
-- +goose StatementEnd
