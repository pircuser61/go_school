-- +goose Up
-- +goose StatementBegin

create or replace function add_check_sla_true(w_id uuid, content jsonb)
    returns jsonb
    language plpgsql
as $function$
declare
    step_names varchar[];
    s_name varchar;
begin
    step_names = array(
        select step_name
            from variable_storage
        where work_id = w_id
            and step_type in ('execution', 'approver')
    );

    foreach s_name IN ARRAY step_names
    loop
        select jsonb_set(content, array['State', s_name, 'check_sla']::varchar[], 'true'::jsonb, true)
            into content;
    end loop;

    return content;

exception when others then
    raise notice 'work_id: %', w_id;
    return content;
end
$function$;

update variable_storage
    set content = add_check_sla_true(work_id, content)
where status = 'running' and step_type not in ('start', 'servicedesk_application');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
create or replace function remove_check_sla_true(w_id uuid, content jsonb)
    returns jsonb
    language plpgsql
as $function$
declare
    step_names varchar[];
    s_name varchar;
begin
    step_names = array(
        select step_name
            from variable_storage
        where work_id = w_id
            and step_type in ('execution', 'approver')
    );

    foreach s_name IN ARRAY step_names
    loop
        select content = content #- array['State', s_name, 'check_sla']::varchar[];
    end loop;

    return content;

exception when others then
    raise notice 'work_id: %', w_id;
    return content;
end
$function$;

update variable_storage
    set content = remove_check_sla_true(work_id, content)
where status = 'running' and step_type not in ('start', 'servicedesk_application');
-- +goose StatementEnd
