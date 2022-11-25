-- +goose Up
-- +goose StatementBegin

ALTER TABLE works
    DROP COLUMN IF EXISTS rate;

ALTER TABLE works
    DROP COLUMN IF EXISTS rate_comment;

ALTER TABLE works
    ADD COLUMN rate integer;

ALTER TABLE works
    ADD COLUMN rate_comment text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN IF EXISTS rate;

ALTER TABLE works
    DROP COLUMN IF EXISTS rate_comment;
-- +goose StatementEnd
