-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION add_signatures_to_output(input_v_ids uuid) RETURNS void
    language plpgsql
AS
$function$
DECLARE
    step_names varchar[];
    s_name     varchar;
BEGIN
    step_names = array(
            SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks')
            FROM versions
            WHERE id = input_v_ids
              AND deleted_at IS NULL
              AND jsonb_typeof(content -> 'pipeline' -> 'blocks') = 'object'
        );

    FOREACH s_name IN ARRAY step_names
        LOOP
            IF s_name LIKE 'sign_%' THEN
                UPDATE pipeliner.public.versions v
                SET "content" = jsonb_set("content"::jsonb,
                                          array ['pipeline', 'blocks', s_name, 'output', 'properties', 'attachments', 'items', 'format']::varchar[],
                                          to_jsonb('file'::text),
                                          true);
                UPDATE pipeliner.public.versions v
                SET "content" = jsonb_set("content"::jsonb,
                                          array ['pipeline', 'blocks', s_name, 'output', 'properties', 'signatures']::varchar[],
                                          jsonb_build_object(
                                                  'type', 'array',
                                                  'global', concat('s_name', '.signatures'),
                                                  'items', jsonb_build_object(
                                                          'properties', jsonb_build_object(
                                                          'file', jsonb_build_object(
                                                                  'description', 'file to sign',
                                                                  'format', 'file',
                                                                  'properties', jsonb_build_object(
                                                                          'external_link', jsonb_build_object(
                                                                                  'description',
                                                                                  'link to file in another system',
                                                                                  'type', 'string'
                                                                              ),
                                                                          'file_id', jsonb_build_object(
                                                                                  'description',
                                                                                  'file id in file Registry',
                                                                                  'type', 'string'
                                                                              )
                                                                      ),
                                                                  'type', 'object'
                                                              ),
                                                          'signature_file', jsonb_build_object(
                                                                  'description', 'signature file',
                                                                  'format', 'file',
                                                                  'properties', jsonb_build_object(
                                                                          'external_link', jsonb_build_object(
                                                                                  'description',
                                                                                  'link to file in another system',
                                                                                  'type', 'string'
                                                                              ),
                                                                          'file_id', jsonb_build_object(
                                                                                  'description',
                                                                                  'file id in file Registry',
                                                                                  'type', 'string'
                                                                              )
                                                                      ),
                                                                  'type', 'object'
                                                              )
                                                      ),
                                                          'type', 'object'
                                                      )),
                                          true)
                WHERE v.id = input_v_ids;
            END IF;
        END LOOP;
END
$function$;

SELECT add_signatures_to_output(v1.id)
FROM pipeliner.public.versions v1
         INNER JOIN (
    SELECT pipeline_id, MAX(created_at) AS last_published
    FROM pipeliner.public.versions
    WHERE status = 2
    GROUP BY pipeline_id
) v2
                    ON v1.pipeline_id = v2.pipeline_id
WHERE v1.created_at >= v2.last_published AND v1.deleted_at IS NULL;

SELECT add_signatures_to_output(v1.id)
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
DROP function IF EXISTS add_signatures_to_output;
-- +goose StatementEnd