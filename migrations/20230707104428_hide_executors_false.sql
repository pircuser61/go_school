-- +goose Up
-- +goose StatementBegin
UPDATE versions
SET content = (REPLACE(content::text, '"hide_executor_from_initiator": true', '"hide_executor_from_initiator": false'))::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
