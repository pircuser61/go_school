-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage ADD check_half_sla boolean default false not null;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage DROP COLUMN check_half_sla;
-- +goose StatementEnd
