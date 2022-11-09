-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS
    count_index
    ON variable_storage(work_id)
    WHERE
                status = ANY('{running,idle,ready}'::text[]) AND
                status <> 'skipped'::text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX count_index
-- +goose StatementEnd
