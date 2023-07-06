-- +goose Up
-- +goose StatementBegin
UPDATE versions
    SET content = REPLACE(content::text, '"type": "SsoPerson"', '"type": "object", "format": "SsoPerson"');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE versions
    SET content = REPLACE(content::text, '"type": "object", "format": "SsoPerson"', '"type": "SsoPerson"');

-- +goose StatementEnd
