-- +goose Up
-- +goose StatementBegin
WITH sub AS (
    SELECT block_id, deadline::text
    FROM deadlines d
             join variable_storage vs on d.block_id = vs.id
)
UPDATE variable_storage
SET content=jsonb_set(content, array ['State', step_name, 'deadline']::varchar[], to_jsonb(sub.deadline), true)
FROM sub
WHERE sub.block_id = id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
