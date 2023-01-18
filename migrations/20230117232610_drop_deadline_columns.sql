-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage
    DROP COLUMN half_sla_deadline;
ALTER TABLE variable_storage
    DROP COLUMN sla_deadline;
ALTER TABLE variable_storage
    DROP COLUMN check_half_sla;
ALTER TABLE variable_storage
    DROP COLUMN check_sla;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage
    ADD COLUMN half_sla_deadline timestamp with time zone;
ALTER TABLE variable_storage
    ADD COLUMN sla_deadline timestamp with time zone;
ALTER TABLE variable_storage
    ADD COLUMN check_half_sla boolean default false not null;
ALTER TABLE variable_storage
    ADD COLUMN check_sla boolean default false not null;
-- +goose StatementEnd
