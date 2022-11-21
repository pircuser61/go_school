-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS variable_storage_step_type_idx ON variable_storage USING btree (step_type);
CREATE INDEX IF NOT EXISTS variable_storage_status_idx ON variable_storage USING btree (status);
CREATE INDEX IF NOT EXISTS variable_storage_work_id_step_type_status_index ON variable_storage USING btree (work_id, step_type, status);
CREATE INDEX IF NOT EXISTS idxgin_content ON variable_storage USING gin (content);
CREATE INDEX IF NOT EXISTS works_started_at ON works USING btree (started_at);
CREATE INDEX IF NOT EXISTS works_work_number_index ON works USING btree (work_number);
CREATE INDEX IF NOT EXISTS works_exp_index_filter ON works USING btree (work_number) WHERE (child_id IS NULL);-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX variable_storage_step_type_idx;
DROP INDEX variable_storage_status_idx;
DROP INDEX variable_storage_work_id_step_type_status_index;
DROP INDEX idxgin_content;
DROP INDEX works_started_at;
DROP INDEX works_work_number_index;
DROP INDEX works_exp_index_filter;
-- +goose StatementEnd
