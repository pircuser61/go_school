-- +goose Up
-- +goose StatementBegin
CREATE TABLE variable_storage_14022024 AS (SELECT * FROM variable_storage);

CREATE TABLE versions_14022024 AS (SELECT * FROM versions);



CREATE OR REPLACE FUNCTION wrap_start(input jsonb) RETURNS jsonb
    LANGUAGE plpgsql
AS $function$
DECLARE
    k text;
    v text;
    resultObj jsonb;
BEGIN
    resultObj = jsonb_set(input, array['start_0.application_body']::text[], '{}', true);

    FOR k, v IN SELECT * FROM jsonb_each(input) LOOP
        IF k LIKE 'start_0.%' AND k NOT IN ('start_0.initiator', 'start_0.workNumber', 'start_0.application_body') THEN
            resultObj = jsonb_set(resultObj, array['start_0.application_body', k]::text[], v::jsonb, true);

            resultObj = jsonb_delete_path(resultObj, array[k]::varchar[]);

            continue;
        END IF;

        resultObj = jsonb_set(resultObj, array[k]::text[], v::jsonb, true);
    END LOOP;

    RETURN resultObj;
END
$function$;

CREATE OR REPLACE FUNCTION wrap_varstore_context(vs_id uuid) RETURNS void
    LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE variable_storage
    SET content = jsonb_set(content,
        array['Values']::varchar[],
        wrap_start(content -> 'Values'),
        true)
    WHERE id = vs_id;
END
$function$;

SELECT wrap_varstore_context(id)
FROM variable_storage
WHERE content ->> 'Values' IS NOT NULL;



UPDATE versions
SET content = replace(content::text, 'start_0.', 'start_0.application_body.')::jsonb
WHERE deleted_at IS NULL;

UPDATE versions
SET content = replace(replace(content::text, 'start_0.application_body.initiator', 'start_0.initiator'),
    'start_0.application_body.workNumber', 'start_0.workNumber')::jsonb
WHERE deleted_at IS NULL;



WITH cte AS (
    SELECT
        id,
        content -> 'pipeline' -> 'blocks' -> 'start_0' -> 'output' -> 'properties' AS props
    FROM versions
    WHERE deleted_at IS NULL
)
UPDATE versions v
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
            'workNumber', cte.props -> 'workNumber')),
    true)
FROM cte
WHERE v.id = cte.id;

UPDATE versions
SET content = content #- '{pipeline, blocks, start_0, output, properties, application_body, properties, initiator}';

UPDATE versions
SET content = content #- '{pipeline, blocks, start_0, output, properties, application_body, properties, workNumber}';



DROP FUNCTION IF EXISTS wrap_start(input jsonb);

DROP FUNCTION IF EXISTS wrap_varstore_context(input jsonb);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
