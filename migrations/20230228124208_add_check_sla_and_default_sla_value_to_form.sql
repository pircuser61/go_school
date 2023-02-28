-- +goose Up
-- +goose StatementBegin

create or replace function add_check_sla_and_default_sla_value_to_form()
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
                    where id = v_id and deleted_at is null and
                            jsonb_typeof(content -> 'pipeline' -> 'blocks') = 'object'
                );

            foreach s_name IN ARRAY step_names
                loop
                    if s_name like 'form%' then
                        update versions
                        set content = jsonb_set(content, array['pipeline', 'blocks', s_name, 'params', 'check_sla']::varchar[], 'false'::jsonb, true)
                        where id = v_id;

                        update versions
                        set content = jsonb_set(content, array['pipeline', 'blocks', s_name, 'params', 'sla']::varchar[], '1'::jsonb, true)
                        where id = v_id;
                    end if;
                end loop;
        end loop;
end
$function$;

select * from add_check_sla_and_default_sla_value_to_form();
-- +goose StatementEnd
