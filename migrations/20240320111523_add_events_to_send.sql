-- +goose Up
-- SQL in this section is executed when the migration is applied
CREATE TABLE events_to_send (
    id uuid NOT NULL,
    work_id uuid NOT NULL REFERENCES works(id),
    message jsonb NOT NULL DEFAULT '{}'::JSONB,
    sent_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT events_to_send_pk PRIMARY KEY (id)
);

COMMENT ON TABLE events_to_send is 'Таблица с ивентами которые не удалось отправить в очередь';

COMMENT ON COLUMN events_to_send.id is 'id записи';
COMMENT ON COLUMN events_to_send.work_id is 'id таски';
COMMENT ON COLUMN events_to_send.message is 'json ивента';
COMMENT ON COLUMN events_to_send.sent_at is 'дата успешной отправки в очередь';
COMMENT ON COLUMN events_to_send.created_at is 'дата создания ивента';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS events_to_send;
