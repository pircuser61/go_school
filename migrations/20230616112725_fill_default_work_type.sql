-- +goose Up
-- +goose StatementBegin
create or replace function add_default_work_type_to_blocks()
    returns void
    language plpgsql
as $function$
declare
    versions uuid[] := array(select distinct id from versions);
    work_type text := '"8/5"';
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
                        set content = jsonb_set(content, array['pipeline', 'blocks', s_name, 'params', 'work_type']::varchar[], work_type::jsonb, true)
                        where id = v_id and jsonb_typeof(content -> 'pipeline' -> 'blocks' -> s_name -> 'params') = 'object';
                    end if;
                    if s_name like 'approver%' then
                        update versions
                        set content = jsonb_set(content, array['pipeline', 'blocks', s_name, 'params', 'work_type']::varchar[], work_type::jsonb, true)
                        where id = v_id and jsonb_typeof(content -> 'pipeline' -> 'blocks' -> s_name -> 'params') = 'object';
                    end if;
                    if s_name like 'execution%' then
                        update versions
                        set content = jsonb_set(content, array['pipeline', 'blocks', s_name, 'params', 'work_type']::varchar[], work_type::jsonb, true)
                        where id = v_id and jsonb_typeof(content -> 'pipeline' -> 'blocks' -> s_name -> 'params') = 'object';
                    end if;
                end loop;
        end loop;
end
$function$;

select * from add_default_work_type_to_blocks();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
