-- +goose Up
-- +goose StatementBegin
comment on schema public is 'Детальные данные о сценариях Jocasta.';

comment on column works.human_status is 'Статус заявки для ServiceDesk.';
comment on column works.child_id is 'Уникальный UUID идентификатор дочерней заявки.';
comment on column works.run_context is 'Контекст исполнения заявки.';

comment on column dict_approve_statuses.title is 'Текст статуса пользовательского действия.';

comment on column dict_approve_statuses.created_at is 'Поле для аудита. Дата создания записи в таблице.';
comment on column dict_approve_statuses.deleted_at is 'Поле для аудита. Дата удаления записи в таблице.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
comment on schema public is null;

comment on column works.human_status is null;
comment on column works.child_id is null;
comment on column works.run_context is null;

comment on column dict_approve_statuses.title is null;

comment on column dict_approve_statuses.created_at is null;
comment on column dict_approve_statuses.deleted_at is null;
-- +goose StatementEnd
