-- +goose Up
-- +goose StatementBegin
    UPDATE versions v
       SET content = jsonb_set(v.content, '{name}', to_jsonb(p.name) , false)
    FROM  pipelines p
    WHERE v.pipeline_id=p.id AND NOT (content ->> 'name') = p.name
        AND p.name LIKE  '% - копия';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
