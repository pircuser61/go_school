-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS pipeline_history_date_index
    ON pipeline_history (date DESC);

CREATE INDEX IF NOT EXISTS variable_storage_time_index
    ON variable_storage (time DESC);

CREATE INDEX IF NOT EXISTS pipelines_created_at_index
    ON pipelines (created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS pipeline_history_date_index;
DROP INDEX IF EXISTS variable_storage_time_index;
DROP INDEX IF EXISTS pipelines_created_at_index;
-- +goose StatementEnd
