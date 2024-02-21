-- +goose Up
-- +goose StatementBegin
UPDATE members
SET finished = false
WHERE finished = true;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
