-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage
    ADD COLUMN current_executor VARCHAR(256) DEFAULT '';

WITH blocks AS (
    SELECT id, content -> 'State' -> step_name AS block_data
    FROM pipeliner.public.variable_storage
    WHERE step_type = 'execution'
      AND status IN ('running', 'idle', 'ready')
)
   , exec_data AS (
    SELECT id
         , block_data ->> 'is_taken_in_work' = 'true'                      AS taken_in_work
         , array_to_string(array(SELECT jsonb_object_keys(block_data -> 'executors')),',') AS first_exec
         , block_data ->> 'executors_group_id'                             AS exec_group
    FROM blocks
       WHERE jsonb_typeof(block_data -> 'executors') = 'object'
)
   , current_executor AS (
    SELECT id,
           CASE
               WHEN taken_in_work
                   THEN coalesce(first_exec, '')
               ELSE coalesce(nullif(exec_group, ''), coalesce(first_exec, '')) END AS executor
    FROM exec_data
)
UPDATE variable_storage
SET current_executor = ce.executor
FROM current_executor ce
WHERE ce.id = variable_storage.id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage
    DROP COLUMN current_executor;
-- +goose StatementEnd
