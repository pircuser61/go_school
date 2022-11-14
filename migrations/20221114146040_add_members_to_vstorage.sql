-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS started_at_pr ON works (started_at);

ALTER TABLE variable_storage
    ADD COLUMN IF NOT EXISTS members varchar[];

UPDATE variable_storage
SET members = array(
    SELECT
            json_object_keys(variable_storage.content::json -> 'State' -> step_name -> 'approvers'
        ) AS keys
    )
WHERE step_type = 'approver'
    AND variable_storage.content -> 'State' -> step_name -> 'approvers' != 'null'::jsonb;

UPDATE variable_storage
SET members = array(
    SELECT
            json_object_keys(variable_storage.content::json -> 'State' -> step_name -> 'executors'
        ) AS keys
    )
WHERE step_type = 'execution'
  AND variable_storage.content -> 'State' -> step_name -> 'executors' != 'null'::jsonb;

UPDATE variable_storage
SET members = array(
    SELECT
            json_object_keys(variable_storage.content::json -> 'State' -> step_name -> 'executors'
        ) AS keys
    )
WHERE step_type = 'form'
  AND variable_storage.content -> 'State' -> step_name -> 'executors' != 'null'::jsonb;

CREATE INDEX IF NOT EXISTS index_members on variable_storage USING GIN ("members");

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS index_members;
DROP INDEX IF EXISTS started_at_pr;
ALTER TABLE variable_storage
    DROP COLUMN IF EXISTS members;
-- +goose StatementEnd
