-- +goose Up
-- +goose StatementBegin

create or replace function add_check_sla_true()
    returns void
    language plpgsql
as $function$
declare
    works uuid[] := array(select distinct work_id from variable_storage;
    step_names varchar[];
    w_id uuid;
    s_name varchar;
begin
    foreach w_id IN ARRAY works
    loop
        step_names = array(
            select step_name
                from variable_storage
            where work_id = w_id
                and step_type in ('execution', 'approver')
        );

        foreach s_name IN ARRAY step_names
        loop
            update variable_storage
            set content = jsonb_set(content, array['State', s_name, 'check_sla']::varchar[], 'true'::jsonb, true)
            where work_id = w_id;
        end loop;
end loop;
end
$function$;

select * from add_check_sla_true();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
create or replace function remove_check_sla_true()
    returns void
    language plpgsql
as $function$
declare
    works uuid[] := array(select distinct work_id from variable_storage;
    step_names varchar[];
    w_id uuid;
    s_name varchar;
begin
    foreach w_id IN ARRAY works
    loop
        step_names = array(
            select step_name
                from variable_storage
            where work_id = w_id
                and step_type in ('execution', 'approver')
        );

        foreach s_name IN ARRAY step_names
        loop
            update variable_storage
            set content = content #- array['State', s_name, 'check_sla']::varchar[]
            where work_id = w_id;
        end loop;
end loop;
end
$function$;

select * from remove_check_sla_true();
-- +goose StatementEnd
