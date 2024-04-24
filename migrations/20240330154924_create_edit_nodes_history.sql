-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS update_inputs_history(
    work_id uuid NOT NULL REFERENCES works(id),
    event_id uuid NOT NULL REFERENCES task_events(id),
    step_name varchar NOT NULL,
    author varchar NOT NULL,
    content jsonb NOT NULL DEFAULT '{}'::JSONB,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT update_inputs_history_pk PRIMARY KEY (work_id, step_name, created_at)
);

comment on table update_inputs_history is 'Таблица с историей измненией инпутов блока';

comment on column update_inputs_history.work_id is 'Уникальный UUID идентификатор таски.';
comment on column update_inputs_history.event_id is 'Уникальный UUID идентификатор события.';
comment on column update_inputs_history.step_name is 'Название блока.';
comment on column update_inputs_history.author is 'Логин автора изменений.';
comment on column update_inputs_history.created_at is 'Дата создания изменения.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS update_inputs_history;
-- +goose StatementEnd
