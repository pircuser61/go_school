-- +goose Up
-- +goose StatementBegin
UPDATE versions v
SET v.content = jsonb_set(v.content,
    array['pipeline', 'blocks', 'start_0', 'output', 'properties', 'application_data']::varchar[],
    jsonb_build_object(),
    true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE versions v
SET v.content = v.content #- '{pipeline, blocks, start_0, output, properties, application_data}';
-- +goose StatementEnd
