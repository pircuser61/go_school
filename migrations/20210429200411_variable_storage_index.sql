-- +goose Up
-- +goose StatementBegin
CREATE INDEX idx_variable_storage_work_id ON variable_storage (work_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_variable_storage_work_id;
-- +goose StatementEnd
