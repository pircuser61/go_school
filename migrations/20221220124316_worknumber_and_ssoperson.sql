-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION replace_version_content(id uuid) RETURNS jsonb AS
$BODY$
DECLARE
    val jsonb;

    form_list text[];
    app_list text[];

    el text; --iterator
BEGIN
    -- get current json
    select v.content::jsonb
    from versions v
    where v.id = $1 into val;

    -- replace forms
    select array_agg(key(data))
    from (select jsonb_each(v.content #> '{pipeline,blocks}') as data
          from versions v
          where v.id = $1) a
    where value(data) ->> 'type_id' = 'form' into form_list;

    IF form_list IS NOT NULL THEN
        FOREACH el IN ARRAY form_list
            LOOP
                select jsonb_set(val, ('{pipeline,blocks,' || el || ',output,0}')::text[], ('{
              "name": "executor",
              "type": "SsoPerson",
              "global": "' || el || '.executor"
            }')::jsonb) into val;
            END LOOP;
    END IF;

    -- replace apps
    select array_agg(key(data))
    from (select jsonb_each(v.content #> '{pipeline,blocks}') as data
          from versions v
          where v.id = $1) a
    where value(data) ->> 'type_id' = 'servicedesk_application' into app_list;

    IF app_list IS NOT NULL THEN
        FOREACH el IN ARRAY app_list
            LOOP
                select jsonb_set(val, ('{pipeline,blocks,' || el || ',output,3}')::text[], ('{
              "name": "executor",
              "type": "SsoPerson",
              "global": "' || el || '.executor"
            }')::jsonb) into val;
            END LOOP;
    END IF;
    RETURN val;

exception when others then
    raise notice 'ID: %', id;
    return val;
END
$BODY$
    LANGUAGE plpgsql;

update versions
set content = jsonb_set(replace_version_content(id), '{pipeline,blocks,start_0,output}', '[{
  "name": "workNumber",
  "type": "string",
  "global": "start_0.workNumber"
}]');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop function replace_version_content;
-- +goose StatementEnd
