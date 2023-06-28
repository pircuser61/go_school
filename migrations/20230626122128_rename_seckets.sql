-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rename_socket_in_version(id uuid, block_type varchar, r_from varchar, r_to varchar) RETURNS jsonb AS
    $BODY$
    DECLARE
    val jsonb;
        block_list text[];
        el text;
        sockets text[];
        suck jsonb;
        suck_index int = 0;
        l_context text;
    BEGIN
    select v.content::jsonb
    from versions v
    where v.id = $1 into val;

    select array_agg(key(data))
    from (select jsonb_each(v.content #> '{pipeline,blocks}') as data
          from versions v
          where v.id = $1) a
    where value(data) ->> 'type_id' = $2 into block_list;

    IF block_list IS NOT NULL THEN
            FOREACH el IN ARRAY block_list
                LOOP
    SELECT ARRAY (SELECT jsonb_array_elements(val -> 'pipeline' -> 'blocks' -> el -> 'sockets')) into sockets;
    IF sockets IS NOT NULL THEN
            suck_index = 0;
            FOREACH suck IN ARRAY sockets
                LOOP
                    IF (suck -> 'id')::text = '"'||$3||'"'::text THEN
        SELECT jsonb_set(val, ('{pipeline,blocks,' || el || ',sockets,' || suck_index::text || ',id}')::text[], ('"'||$4||'"')::jsonb, false) into val;
        END IF;
        suck_index = suck_index + 1;
    END LOOP;
END IF;
END LOOP;
END IF;

RETURN val;

exception when others then
    raise notice 'ID: %', id;
    GET STACKED DIAGNOSTICS l_context = PG_EXCEPTION_CONTEXT;
    RAISE NOTICE 'ERROR:%', l_context;
return val;
END
$BODY$
LANGUAGE plpgsql;

update versions
set content = rename_socket_in_version(id, 'approver', 'edit_app', 'approver_send_edit_app');

update versions
set content = rename_socket_in_version(id, 'execution', 'edit_app', 'executor_send_edit_app');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
