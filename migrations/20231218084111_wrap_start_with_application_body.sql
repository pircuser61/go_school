-- +goose Up
-- +goose StatementBegin
CREATE TABLE versions_21122023 AS (SELECT * FROM pipeliner.public.versions);

CREATE TABLE variable_storage_21122023 AS (SELECT * FROM pipeliner.public.variable_storage);



CREATE OR REPLACE FUNCTION jsonb_replace_obj_mapping(obj jsonb, path varchar[], input_v_ids uuid) RETURNS void
    language plpgsql AS $function$
DECLARE
    step_mappings varchar[];
    s_map varchar;
BEGIN
    step_mappings = array(SELECT jsonb_object_keys(obj));

    FOREACH s_map IN ARRAY step_mappings
        loop
            if obj -> s_map ->> 'value' LIKE 'start_0%' then
                UPDATE pipeliner.public.versions v
                SET "content" = jsonb_set("content"::jsonb,
                    array_cat(path, array[s_map, 'value']::varchar[]),
                    jsonb_build_array(concat(
                        left(obj -> s_map ->> 'value', 8),
                        'application_body.',
                        substring(obj -> s_map ->> 'value', 9, length(obj -> s_map ->> 'value')))) -> 0,
                    false)
                WHERE v.id = input_v_ids;
            end if;

            if obj -> s_map -> 'properties' IS NOT NULL then
                PERFORM jsonb_replace_obj_mapping(
                    obj -> s_map -> 'properties',
                    array_cat(path, array[s_map, 'properties']::varchar[]),
                    input_v_ids);
            end if;
        end loop;
END $function$;

CREATE OR REPLACE FUNCTION add_app_body_prefix_mapping(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    step_names varchar[];
    s_name varchar;
BEGIN
    step_names = array(
        SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks')
        FROM pipeliner.public.versions
        WHERE id = input_v_ids);

    FOREACH s_name IN ARRAY step_names
        LOOP
            PERFORM jsonb_replace_obj_mapping(
                (
                    SELECT content -> 'pipeline' -> 'blocks' -> s_name -> 'params' -> 'mapping'
                    FROM pipeliner.public.versions
                    WHERE id = input_v_ids
                        AND jsonb_typeof(content -> 'pipeline' -> 'blocks' -> s_name -> 'params' -> 'mapping') = 'object'
                ),
                array['pipeline', 'blocks', s_name, 'params', 'mapping']::varchar[],
                input_v_ids
            );
        END LOOP;
END $function$;

SELECT add_app_body_prefix_mapping(id)
FROM pipeliner.public.versions
WHERE deleted_at IS NULL
    AND jsonb_typeof(content -> 'pipeline' -> 'blocks') = 'object';



WITH cte AS (
    SELECT
        pipeline_id,
        content -> 'pipeline' -> 'blocks' -> 'start_0' -> 'output' -> 'properties' AS props
    FROM pipeliner.public.versions
)
UPDATE pipeliner.public.versions v
SET content = jsonb_set(content,
    array['pipeline', 'blocks', 'start_0', 'output']::varchar[],
    jsonb_build_object(
        'type', 'object',
        'properties', jsonb_build_object(
            'application_body', jsonb_build_object(
                'title', 'Application body',
                'type', 'object',
                'global', 'start_0.application_body',
                'properties', cte.props),
            'initiator', cte.props -> 'initiator',
            'workNumber', cte.props -> 'workNumber')
    ),
    true)
FROM cte
WHERE v.pipeline_id = cte.pipeline_id;

UPDATE pipeliner.public.versions
SET content = content #- '{pipeline, blocks, start_0, output, properties, application_body, properties, initiator}';

UPDATE pipeliner.public.versions
SET content = content #- '{pipeline, blocks, start_0, output, properties, application_body, properties, workNumber}';



CREATE OR REPLACE FUNCTION add_app_body_prefix_output(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    s_mappings varchar[];
    s_map varchar;
    str_to_update varchar;
BEGIN
    s_mappings = array(
        SELECT jsonb_object_keys(
            content -> 'pipeline' -> 'blocks' -> 'start_0' -> 'output' -> 'properties' -> 'application_body' -> 'properties')
        FROM pipeliner.public.versions
        WHERE id = input_v_ids AND deleted_at IS NULL);

    FOREACH s_map IN ARRAY s_mappings
        LOOP
            str_to_update = (
                SELECT content -> 'pipeline' -> 'blocks' -> 'start_0' -> 'output' -> 'properties' -> 'application_body' -> 'properties' -> s_map ->> 'global'
                FROM pipeliner.public.versions
                WHERE id = input_v_ids AND deleted_at IS NULL);

            UPDATE pipeliner.public.versions v
            SET "content" = jsonb_set("content"::jsonb,
                array['pipeline', 'blocks', 'start_0', 'output', 'properties', 'application_body', 'properties', s_map, 'global']::varchar[],
                jsonb_build_array(concat(
                    left(str_to_update, 8),
                    'application_body.',
                    substring(str_to_update, 9, length(str_to_update)))) -> 0,
                false)
            WHERE v.id = input_v_ids;
        END LOOP;
END $function$;

SELECT add_app_body_prefix_output(id)
FROM pipeliner.public.versions;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
