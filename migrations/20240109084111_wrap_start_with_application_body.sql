-- +goose Up
-- +goose StatementBegin
CREATE TABLE versions_09012024 AS (SELECT * FROM pipeliner.public.versions);

CREATE TABLE variable_storage_09012024 AS (SELECT * FROM pipeliner.public.variable_storage);



UPDATE pipeliner.public.versions
SET content = replace(content::text, 'start_0.', 'start_0.application_body.')::jsonb
WHERE deleted_at IS NULL;

UPDATE pipeliner.public.versions
SET content = replace(replace(
        content::text, 'start_0.application_body.initiator', 'start_0.initiator'),
            'start_0.application_body.workNumber', 'start_0.workNumber')::jsonb
WHERE deleted_at IS NULL;



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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM versions;

INSERT INTO versions
SELECT *
FROM versions_09012024;



DELETE FROM variable_storage;

INSERT INTO variable_storage
SELECT *
FROM variable_storage_09012024;
-- +goose StatementEnd
