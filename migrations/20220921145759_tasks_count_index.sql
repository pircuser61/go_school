-- +goose Up
-- +goose StatementBegin
CREATE INDEX CONCURRENTLY IF NOT EXISTS
    count_index
    ON pipeliner.variable_storage(work_id)
    WHERE
                status = ANY('{running,idle,ready}'::text[]) AND
                status <> 'skipped'::text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX pipeliner.count_index
-- +goose StatementEnd
