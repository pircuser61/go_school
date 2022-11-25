-- +goose Up
-- +goose StatementBegin
create table if not exists dict_actions(
    id character varying not null,
    title character varying,
    default_priority character varying,
    comment_enabled bool default false not null,
    attachments_enabled bool default false not null,

    constraint dict_actions_pkey primary key (id)
);

insert into dict_actions(id,
                         title,
                         default_priority,
                         comment_enabled,
                         attachments_enabled)
values
-- executors
('execution', 'Решить', 'primary', true, false),
('executor_start_work', 'Взять в работу', 'primary', false, false),
('change_executor', 'Переназначить', 'other', true, false),
('request_execution_info', 'Запросить информацию', 'other', true, true),
-- form
('fill_form', '', '', false, false), -- "fill_form"
-- approvers
('add_approvers', 'Добавить согласующего', 'other', true, false),
('request_add_info', 'Запросить информацию', 'secondary', true, true),
('send_edit_app', 'Вернуть на доработку', 'other', true, true),
('approve', 'Согласовать', 'primary', true, false),
('reject', 'Отклонить', 'secondary', true, false),
('viewed', 'Ознакомлен', 'other', true, false),
('informed', 'Проинформирован', 'other', true, false),
('sign', 'Подписать', 'other', true, false),
('affirmate', 'Утвердить', 'other', true, false),
-- misc
('cancel_app', 'Отозвать', 'other', true, false);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists dict_actions;
-- +goose StatementEnd
