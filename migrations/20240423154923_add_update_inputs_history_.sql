-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS task_steps_inputs(
    work_id uuid NOT NULL REFERENCES works(id),
    event_id uuid NOT NULL REFERENCES task_events(id),
    step_name varchar NOT NULL,
    author varchar NOT NULL,
    content jsonb NOT NULL DEFAULT '{}'::JSONB,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT update_inputs_history_pk PRIMARY KEY (work_id, step_name, created_at)
);

comment on table task_steps_inputs is 'Таблица с историей измненией инпутов блока';

comment on column task_steps_inputs.work_id is 'Уникальный UUID идентификатор таски.';
comment on column task_steps_inputs.event_id is 'Уникальный UUID идентификатор события.';
comment on column task_steps_inputs.step_name is 'Название блока.';
comment on column task_steps_inputs.author is 'Логин автора изменений.';
comment on column task_steps_inputs.created_at is 'Дата создания изменения.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS task_steps_inputs;
-- +goose StatementEnd