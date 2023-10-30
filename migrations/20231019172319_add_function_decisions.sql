-- +goose Up
-- +goose StatementBegin
INSERT INTO dict_node_decisions (id, node_type, decision, title)
VALUES
    (uuid_generate_v4(), 'executable_function', 'executed', 'Исполнено'),
    (uuid_generate_v4(), 'executable_function', 'timeout', 'Просрочено');

CREATE OR REPLACE FUNCTION add_executable_function_output(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    step_names varchar[];
    s_name varchar;
BEGIN
    step_names = array(
            SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks')
            FROM versions
            WHERE id = input_v_ids AND deleted_at IS NULL
              AND jsonb_typeof(content -> 'pipeline' -> 'blocks') = 'object'
        );

    FOREACH s_name IN ARRAY step_names
        LOOP
            IF s_name LIKE 'executable_function%' THEN
                UPDATE pipeliner.public.versions v
                SET "content" = jsonb_set("content"::jsonb,
                    array['pipeline', 'blocks', s_name, 'output', 'properties', 'decision']::varchar[],
                    jsonb_build_object(
                        'type', 'string',
                        'title', 'Решение',
                        'global', concat(s_name, '.decision')),
                    true)
                WHERE v.id = input_v_ids;
            END IF;
        END LOOP;
END $function$;

SELECT add_executable_function_output(v1.id)
FROM pipeliner.public.versions v1
INNER JOIN (
    SELECT pipeline_id, MAX(created_at) AS last_published
    FROM pipeliner.public.versions
    WHERE status = 2
    GROUP BY pipeline_id
) v2
    ON v1.pipeline_id = v2.pipeline_id
WHERE v1.created_at >= v2.last_published AND v1.deleted_at IS NULL;

SELECT add_executable_function_output(v1.id)
FROM pipeliner.public.versions v1
INNER JOIN (
    SELECT pipeline_id, MAX(status)
    FROM pipeliner.public.versions
    GROUP BY pipeline_id
    HAVING MAX(status) = 1
) v2
    ON v1.pipeline_id = v2.pipeline_id
WHERE v1.deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM dict_node_decisions
WHERE node_type = 'function'
  AND decision IN (
          'executed', 'timeout'
        );

DROP function IF EXISTS add_executable_function_output;
-- +goose StatementEnd
