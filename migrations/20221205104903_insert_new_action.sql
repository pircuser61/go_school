-- +goose Up
-- +goose StatementBegin
insert into dict_approve_action_names (id, title, status_processing_title, status_decision_title, created_at)
values ('approver_send_edit_app', 'На доработку', '', '', now())
on conflict do nothing;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
delete from dict_approve_action_names
where id = 'approver_send_edit_app';
-- +goose StatementEnd
