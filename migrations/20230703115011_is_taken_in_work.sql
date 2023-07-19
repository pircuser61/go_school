-- +goose Up
-- +goose StatementBegin
update variable_storage
set content = jsonb_set(content, array ['State', step_name, 'is_taken_in_work']::varchar[], 'true'::jsonb, true)
where id in (
    select id
    from variable_storage
    where step_type in ('execution', 'form')
      and content -> 'State' -> step_name ->> 'is_taken_in_work' != 'true'
      and content -> 'State' -> step_name ->> 'executors' != 'null'
      and array_length(array(select jsonb_object_keys(content -> 'State' -> step_name -> 'executors')), 1) = 1
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
