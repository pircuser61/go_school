-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN IF NOT EXISTS version_sla_id uuid REFERENCES version_sla (id);

comment on column works.version_sla_id is 'id sla из настроек процесса';

-- для тех, что были созданы тогда, когда sla уже был
WITH vsla AS (SELECT id, created_at, version_id FROM version_sla ORDER BY created_at DESC)
UPDATE works
SET version_sla_id = (
    SELECT id
    FROM vsla
    WHERE vsla.version_id = works.version_id
      AND vsla.created_at <= works.started_at
    LIMIT 1
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN IF EXISTS version_sla_id;
-- +goose StatementEnd
