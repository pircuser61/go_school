-- +goose Up
-- +goose StatementBegin
alter table dict_approve_action_names
    alter column id type varchar;

update dict_approve_action_names
set id = case
     when id = '82f2324d-cea1-4024-99c1-674380483d39' then 'approved'
     when id = '55fe7832-9109-45b0-883b-cfacc25d14ca' then 'rejected'
     when id = 'a747532c-8a9d-42c7-98cc-07a341ca41c6' then 'confirm'
     when id = 'cf75561b-965a-46d5-a806-b8d59d9bc69e' then 'viewed'
     when id = '96cdb5f7-d9af-453d-9292-f9d87339a059' then 'informed'
     when id = '43d16439-f7e3-4dbb-8431-3bd401f46d9b' then 'sign'
end;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table dict_approve_action_names
    alter column id type uuid;

update dict_approve_action_names
set id = case
     when id = 'approved' then '82f2324d-cea1-4024-99c1-674380483d39'
     when id = 'rejected' then '55fe7832-9109-45b0-883b-cfacc25d14ca'
     when id = 'confirm' then 'a747532c-8a9d-42c7-98cc-07a341ca41c6'
     when id = 'viewed' then 'cf75561b-965a-46d5-a806-b8d59d9bc69e'
     when id = 'informed' then '96cdb5f7-d9af-453d-9292-f9d87339a059'
     when id = 'sign' then '43d16439-f7e3-4dbb-8431-3bd401f46d9b'
end;

insert into dict_approve_action_names (id, title, status_processing_title, status_decision_title, created_at)
values ('approver_send_edit_app', 'На доработку', '', '', now())
-- +goose StatementEnd
