-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage
    ADD COLUMN has_error bool NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage
    DROP COLUMN has_error;
-- +goose StatementEnd

