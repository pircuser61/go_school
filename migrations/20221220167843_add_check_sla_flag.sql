-- +goose Up
-- +goose StatementBegin

create or replace function add_check_sla()
returns void
language plpgsql
as $function$
declare
    ids_array integer;
    i integer;
begin
    select step_name from variable_storage into ids_array;

    foreach i IN ARRAY ids_array
    loop
        update variable_storage
            set content = jsonb_set(content, '{State,execution_0,check_sla}', 'true'::jsonb, true)
        where id = '3f5443d2-6bfc-46a2-962e-ca4da1f41d0b'
    end loop;
end;
as $function$

select * from add_check_sla();

-- +goose StatementEnd