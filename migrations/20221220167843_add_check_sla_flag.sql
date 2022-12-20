-- +goose Up
-- +goose StatementBegin

create or replace function add_check_sla()
returns void
language plpgsql
as $function$
declare
    blocks []uuid;
    execution_keys []varchar;
    approver_keys []varchar;
    i integer;
begin
    select id from variable_storage into blocks;

    foreach i IN ARRAY blocks
    loop
        update variable_storage
            set content = jsonb_set(content, '{State,execution_0,check_sla}', 'true'::jsonb, true)
        where id = blocks[i]
    end loop;
end;
as $function$

select * from add_check_sla();

-- +goose StatementEnd