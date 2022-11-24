-- +goose Up
-- +goose StatementBegin
alter table pipeliner.dict_approve_action_names
    alter column id type varchar;

alter table pipeliner.dict_approve_action_names
    add block_type varchar,
    add default_priority varchar;

update pipeliner.dict_approve_action_names
set id = case
             when '82f2324d-cea1-4024-99c1-674380483d39' then 'approve'
             when '55fe7832-9109-45b0-883b-cfacc25d14ca' then 'reject'
             when 'a747532c-8a9d-42c7-98cc-07a341ca41c6' then 'affirmate'
             when 'cf75561b-965a-46d5-a806-b8d59d9bc69e' then 'viewed'
             when '96cdb5f7-d9af-453d-9292-f9d87339a059' then 'informed'
             when '43d16439-f7e3-4dbb-8431-3bd401f46d9b' then 'sign'
    end;

insert into pipeliner.dict_approve_action_names (id,
                                                 title,
                                                 status_processing_title,
                                                 status_decision_title,
                                                 block_type,
                                                 default_priority,
                                                 created_at)
values
('send_edit_app', 'Отправить на доработку', 'На согласовании', '', 'approver', 'extra', now()),
('add_approvers', 'Добавить согласующего', 'На согласовании', '', 'approver', 'extra', now()),
('request_add_info', '', '', '', 'approver', '', now()),

('cancel_app', 'Отозвать', '', 'Отозвано', '', 'extra', now()),

('executor_start_work', 'Взять в работу', '', 'Взято в работу', 'execution', 'main', now()),
('change_executor', 'Изменить исполнителя', '', 'Взято в работу', 'execution', '', now()),
('request_execution_info', '', '', '', 'execution', '', now()),

('fill_form', '', 'Заполнение данных', '', 'form', '', now());

update pipeliner.dict_approve_action_names as dict
set
   block_type = block_type,
   default_priority = default_priority
from (
         values
         ('approve', 'approver', 'main'),
         ('affirmate', 'approver', 'main'),
         ('viewed', 'approver', 'main'),
         ('affirmate', 'approver', 'main'),
         ('informed', 'approver', 'main'),
         ('sign', 'approver', 'main'),
         ('reject', 'approver', 'secondary')
     ) as c(id, block_type, default_priority)
where c.id = dict.id;

alter table pipeliner.dict_approve_action_names
    rename to dict_action_names;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table pipeliner.dict_approve_action_names
    alter column id type uuid;

alter table pipeliner.dict_approve_action_names
    drop column block_type,
    drop column default_priority;

update pipeliner.dict_approve_action_names
set id = case
             when 'approve' then '82f2324d-cea1-4024-99c1-674380483d39'
             when 'reject' then '55fe7832-9109-45b0-883b-cfacc25d14ca'
             when 'affirmate' then 'a747532c-8a9d-42c7-98cc-07a341ca41c6'
             when 'viewed' then 'cf75561b-965a-46d5-a806-b8d59d9bc69e'
             when 'informed' then '96cdb5f7-d9af-453d-9292-f9d87339a059'
             when 'sign' then '43d16439-f7e3-4dbb-8431-3bd401f46d9b'
    end;

delete from pipeliner.dict_approve_action_names
where id in ('send_edit_app', 'change_executor', 'cancel_app', 'executor_start_work', 'fill_form');

alter table pipeliner.dict_action_names
    rename to dict_approve_action_names;
-- +goose StatementEnd
