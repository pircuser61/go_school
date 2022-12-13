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
('approver_send_edit_app', 'Вернуть на доработку', true, true, true),
('approve', 'Согласовать', true, true, true),
('reject', 'Отклонить', true, true, false),
('viewed', 'Ознакомлен', true, true, true),
('informed', 'Проинформирован', true, true, true),
('sign', 'Подписать', true, true, true),
('confirm', 'Утвердить', true, true, true),
-- misc
('cancel_app', 'Отозвать', true, true, false),

('add_approvers', 'Добавить согласующего', true, true, true),
('additional_approvement', 'Согласовать', true, true, true),
('additional_reject', 'Отклонить', true, true, true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists dict_actions;
-- +goose StatementEnd
