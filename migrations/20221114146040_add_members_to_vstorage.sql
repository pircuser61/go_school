-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS started_at_pr ON pipeliner.works (started_at);

ALTER TABLE pipeliner.variable_storage
    ADD COLUMN IF NOT EXISTS members varchar[];

UPDATE pipeliner.variable_storage
SET members = array(
    SELECT
            json_object_keys(pipeliner.variable_storage.content::json -> 'State' -> step_name -> 'approvers'
        ) AS keys
    )
WHERE step_type = 'approver';

UPDATE pipeliner.variable_storage
SET members = array(
    SELECT
            json_object_keys(pipeliner.variable_storage.content::json -> 'State' -> step_name -> 'executors'
        ) AS keys
    )
WHERE step_type = 'execution';

UPDATE pipeliner.variable_storage
SET members = array(
    SELECT
            json_object_keys(pipeliner.variable_storage.content::json -> 'State' -> step_name -> 'executors'
        ) AS keys
    )
WHERE step_type = 'form';

CREATE INDEX IF NOT EXISTS index_members on pipeliner.variable_storage USING GIN ("members");

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS index_members;
DROP INDEX IF EXISTS started_at_pr;
ALTER TABLE pipeliner.variable_storage
    DROP COLUMN IF EXISTS members;
-- +goose StatementEnd
