 -- +goose Up
-- +goose StatementBegin
 update variable_storage
 set content = jsonb_set(content, array ['State', step_name, 'work_type']::varchar[],
                         '"8/5"'::jsonb, true)
 where step_type in ('form', 'execution', 'approver');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
