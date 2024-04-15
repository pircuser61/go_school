-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
ALTER COLUMN work_number TYPE text NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
ALTER COLUMN work_number TYPE text default ('J'::text ||
         to_char(nextval('work_seq'::regclass), 'fm00000000000000'::text)) NOT NULL
-- +goose StatementEnd
