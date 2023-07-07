-- +goose Up
-- +goose StatementBegin
UPDATE versions
    SET content = REPLACE(content::text, '"type": "SsoPerson"', '"type": "object", "format": "SsoPerson"')::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE versions
    SET content = REPLACE(content::text, '"type": "object", "format": "SsoPerson"', '"type": "SsoPerson"')::jsonb;

-- +goose StatementEnd
