-- +goose Up
-- +goose StatementBegin
UPDATE variable_storage
SET updated_at = time
WHERE status IN ('finished', 'no_success', 'error')
  AND updated_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
