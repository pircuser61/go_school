-- +goose Up
-- +goose StatementBegin
CREATE INDEX idx_variable_storage_work_id ON pipeliner.variable_storage (work_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX pipeliner.idx_variable_storage_work_id;
-- +goose StatementEnd
