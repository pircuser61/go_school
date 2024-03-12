-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage
    ADD COLUMN current_executor jsonb DEFAULT '{}';

WITH blocks AS (
    SELECT id, content -> 'State' -> step_name AS block_data
    FROM pipeliner.public.variable_storage
    WHERE step_type = 'execution'
      AND status IN ('running', 'idle', 'ready')
)
   , exec_data AS (
    SELECT id
         , block_data ->> 'is_taken_in_work' = 'true'                         AS taken_in_work
         , array(SELECT jsonb_object_keys(block_data -> 'executors'))         AS people
         , array(SELECT jsonb_object_keys(block_data -> 'initial_executors')) AS initial_people
         , coalesce(block_data ->> 'executors_group_id', '')                  AS exec_group_id
         , coalesce(block_data ->> 'executors_group_name', '')                AS exec_group_name
    FROM blocks
    WHERE jsonb_typeof(block_data -> 'executors') = 'object' and jsonb_typeof(block_data -> 'initial_executors') = 'object'
)
   , current_executor AS (
    SELECT id,
           jsonb_build_object(
               'group_id', exec_group_id,
               'group_name', exec_group_name,
               'people', people,
               'initial_people', initial_people
            ) AS executor
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
