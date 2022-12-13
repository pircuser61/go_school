-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage ADD COLUMN half_sla_deadline timestamp with time zone;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage DROP COLUMN half_sla_deadline;
-- +goose StatementEnd
