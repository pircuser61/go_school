-- +goose Up
-- +goose StatementBegin
UPDATE versions
    SET content = REPLACE(content::text, '"type": "SsoPerson"', '"type": "Object", "format": "SsoPerson"');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE versions
    SET content = REPLACE(content::text, '"type": "Object", "format": "SsoPerson"', '"type": "SsoPerson"');

-- +goose StatementEnd
