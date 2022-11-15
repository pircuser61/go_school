-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage
    ADD COLUMN break_points text[];
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage
DROP COLUMN break_points;
-- +goose StatementEnd
