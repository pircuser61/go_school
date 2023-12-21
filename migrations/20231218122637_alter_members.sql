-- +goose Up
-- +goose StatementBegin
ALTER TABLE members
    ADD COLUMN IF NOT EXISTS is_initiator not null default false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE members
    DROP COLUMN IF EXISTS is_initiator;
-- +goose StatementEnd
