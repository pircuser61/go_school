-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage
    ADD COLUMN check_sla BOOL NOT NULL DEFAULT FALSE;
ALTER TABLE variable_storage
    ADD COLUMN sla_deadline TIMESTAMP WITH TIME ZONE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage
    DROP COLUMN sla_deadline;
ALTER TABLE variable_storage
    DROP COLUMN check_sla;
-- +goose StatementEnd
