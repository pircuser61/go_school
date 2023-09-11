-- +goose Up
-- +goose StatementBegin
update works set run_context = regexp_replace(run_context::text, '"attachment:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})"', '{"file_id": "\1"}')::jsonb;
update variable_storage set content = regexp_replace(content::text, '"attachment:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})"', '{"file_id": "\1"}')::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
