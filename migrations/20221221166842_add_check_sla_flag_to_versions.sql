-- +goose Up
-- +goose StatementBegin

create or replace function add_check_sla_true_versions()
    returns void
    language plpgsql
as $function$
declare
    versions uuid[] := array(select id from versions);
    step_names varchar[];
    v_id uuid;
    s_name varchar;
begin
    foreach v_id IN ARRAY versions
        loop
            step_names = array(
                select jsonb_object_keys(content -> 'pipeline' -> 'blocks')
                from versions
                where id = v_id
            );

            foreach s_name IN ARRAY step_names
                loop
                    if s_name like 'execution%' or s_name like 'approver%' then
                        update versions
                        set content = jsonb_set(content, array['pipeline', 'blocks', s_name, 'params', 'check_sla']::varchar[], 'true'::jsonb, true)
                        where id = v_id;
                    end if;
                end loop;
        end loop;
end
$function$;

select * from add_check_sla_true_versions();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
create or replace function remove_check_sla_true_versions()
    returns void
    language plpgsql
as $function$
declare
versions uuid[] := array(select distinct id from versions);
    step_names varchar[];
    v_id uuid;
    s_name varchar;
begin
    foreach v_id IN ARRAY versions
        loop
            step_names = array(
                select jsonb_object_keys(content -> 'pipeline' -> 'blocks')
                from versions
                where id = v_id
            );

            foreach s_name IN ARRAY step_names
                loop
                    if s_name like 'execution%' or s_name like 'approver%' then
                        update versions
                        set content = content #- array['pipeline', 'blocks', s_name, 'params', 'check_sla']::varchar[]
                        where id = v_id;
                    end if;
                end loop;
        end loop;
end
$function$;

select * from remove_check_sla_true_versions();
-- +goose StatementEnd
