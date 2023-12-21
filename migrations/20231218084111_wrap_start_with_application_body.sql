-- +goose Up
-- +goose StatementBegin
CREATE TABLE versions_21122023 AS (SELECT * FROM pipeliner.public.versions);

CREATE OR REPLACE FUNCTION add_app_body_prefix_mapping(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    step_names varchar[];
    s_name varchar;
    s_mappings varchar[];
    s_map varchar;
    str_to_update varchar;
BEGIN
    step_names = array(
        SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks')
        FROM pipeliner.public.versions
        WHERE id = input_v_ids AND deleted_at IS NULL
            AND jsonb_typeof(content -> 'pipeline' -> 'blocks') = 'object');

    FOREACH s_name IN ARRAY step_names
        LOOP
            s_mappings = array(
                SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks' -> s_name -> 'params' -> 'mapping')
                FROM pipeliner.public.versions
                WHERE id = input_v_ids AND deleted_at IS NULL
                    AND jsonb_typeof(content -> 'pipeline' -> 'blocks' -> s_name -> 'params' -> 'mapping') = 'object');

            FOREACH s_map IN ARRAY s_mappings
                LOOP
                    str_to_update = (
                        SELECT content -> 'pipeline' -> 'blocks' -> s_name -> 'params' -> 'mapping' -> s_map ->> 'value'
                        FROM pipeliner.public.versions
                        WHERE id = input_v_ids AND deleted_at IS NULL);

                    IF str_to_update LIKE 'start_0%' THEN
                        UPDATE pipeliner.public.versions v
                        SET "content" = jsonb_set("content"::jsonb,
                            array['pipeline', 'blocks', s_name, 'params', 'mapping', s_map, 'value']::varchar[],
                            jsonb_build_array(concat(
                                left(str_to_update, 8),
                                'application_body.',
                                substring(str_to_update, 9, length(str_to_update)))) -> 0,
                            false)
                        WHERE v.id = input_v_ids;
                    END IF;
                END LOOP;
        END LOOP;
END $function$;

SELECT add_app_body_prefix_mapping(id)
FROM pipeliner.public.versions;



CREATE OR REPLACE FUNCTION add_app_body_prefix_output(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    s_mappings varchar[];
    s_map varchar;
    str_to_update varchar;
BEGIN
    s_mappings = array(
        SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks' -> 'start_0' -> 'output' -> 'properties')
        FROM pipeliner.public.versions
        WHERE id = input_v_ids AND deleted_at IS NULL);

    FOREACH s_map IN ARRAY s_mappings
        LOOP
            str_to_update = (
                SELECT content -> 'pipeline' -> 'blocks' -> 'start_0' -> 'output' -> 'properties' -> s_map ->> 'global'
                FROM pipeliner.public.versions
                WHERE id = input_v_ids);

            UPDATE pipeliner.public.versions v
            SET "content" = jsonb_set("content"::jsonb,
                array['pipeline', 'blocks', 'start_0', 'output', 'properties', s_map, 'global']::varchar[],
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
                'properties', cte.props))),
    true)
FROM cte
WHERE v.pipeline_id = cte.pipeline_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE pipeliner.public.versions;

ALTER TABLE versions_21122023 RENAME TO versions;
-- +goose StatementEnd
