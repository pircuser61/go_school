-- +goose Up
-- SQL in this section is executed when the migration is applied

ALTER TABLE variable_storage
    ADD COLUMN IF NOT EXISTS is_paused bool not null default false;

ALTER TABLE works
    ADD COLUMN IF NOT EXISTS is_paused bool not null default false;

COMMENT ON COLUMN variable_storage.is_paused is 'Блок на паузе';
COMMENT ON COLUMN works.is_paused is 'Блок на паузе';

CREATE INDEX variable_storage_is_paused_index
    ON variable_storage (is_paused);

CREATE INDEX works_is_paused_index
    ON works (is_paused);

CREATE TABLE task_events (
                             id uuid NOT NULL,
                             work_id uuid NOT NULL REFERENCES works(id),
                             author character varying NOT NULL,
                             event_type character varying NOT NULL,
                             params jsonb NOT NULL DEFAULT '{}'::JSONB,
                             created_at timestamp with time zone NOT NULL,
                             CONSTRAINT task_events_pk PRIMARY KEY (id)
);

COMMENT ON TABLE task_events is 'Таблица с ивентами таски.';

COMMENT ON COLUMN task_events.id is 'id записи';
COMMENT ON COLUMN task_events.work_id is 'id таски';
COMMENT ON COLUMN task_events.author is 'логин автора ивента';
COMMENT ON COLUMN task_events.event_type is 'тип ивента, start, paused, restart, finish, etc...';
COMMENT ON COLUMN task_events.created_at is 'дата создания ивента';
COMMENT ON COLUMN task_events.params is 'параметры ивента';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP INDEX IF EXISTS variable_storage_is_paused_index;
DROP INDEX IF EXISTS works_is_paused_index;

ALTER TABLE variable_storage
    DROP COLUMN IF EXISTS is_paused;

ALTER TABLE works
    DROP COLUMN IF EXISTS is_paused;

DROP TABLE IF EXISTS task_events;
