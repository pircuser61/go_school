-- +goose Up
-- +goose StatementBegin
insert into dict_approve_action_names (id, title, status_processing_title, status_decision_title, created_at)
values ('sign_ukep', 'Подписать УКЭП', 'На подписании УКЭП',  'Подписано', now())
    on conflict do nothing;

insert into dict_actions(id, title, is_public, comment_enabled, attachments_enabled)
values  ('sign_ukep', 'Подписать УКЭП', true, true, true);

insert into dict_approve_statuses (id, title, created_at)
values ('cdc14d83-99f3-4982-89fa-050681521402', 'На подписании УКЭП', now());
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
delete from dict_approve_action_names where id = 'sign_ukep';
delete from dict_actions where id = 'sign_ukep';
delete from dict_approve_statuses where id = 'cdc14d83-99f3-4982-89fa-050681521402';
-- +goose StatementEnd
