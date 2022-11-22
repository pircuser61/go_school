-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN rate integer NOT NULL DEFAULT 0;

ALTER TABLE works
    ADD COLUMN rate_comment text NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN IF EXISTS rate;

ALTER TABLE works
    DROP COLUMN IF EXISTS rate_comment;
-- +goose StatementEnd
