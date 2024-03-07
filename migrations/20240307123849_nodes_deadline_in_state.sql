-- +goose Up
-- +goose StatementBegin
WITH sub AS (
    SELECT block_id, to_char(deadline, 'YYYY-MM-DD"T"HH:MM:SS"Z"') t
    FROM deadlines d
             join variable_storage vs on d.block_id = vs.id
    WHERE action = 'sla_breached'
)
UPDATE variable_storage
SET content=jsonb_set(content, array ['State', step_name, 'deadline']::varchar[], to_jsonb(sub.t), true)
FROM sub
WHERE sub.block_id = id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
