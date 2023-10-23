-- +goose Up
-- +goose StatementBegin
ALTER TABLE external_systems
    ADD COLUMN IF NOT EXISTS allow_run_as_others boolean NOT NULL DEFAULT false;

ALTER TABLE works
    ADD COLUMN IF NOT EXISTS real_author VARCHAR(256) DEFAULT '';

UPDATE works SET real_author = author
    WHERE real_author = '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE external_systems
    DROP COLUMN IF EXISTS allow_run_as_others;

ALTER TABLE works
    DROP COLUMN IF EXISTS real_author;
-- +goose StatementEnd
