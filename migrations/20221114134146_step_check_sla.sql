-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage ADD COLUMN check_sla BOOL NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage DROP COLUMN check_sla;
-- +goose StatementEnd
