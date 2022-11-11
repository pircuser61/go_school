-- +goose Up
-- +goose StatementBegin
CREATE SEQUENCE work_seq START WITH 1;
ALTER TABLE works ADD COLUMN work_number TEXT NOT NULL DEFAULT 'J' || TO_CHAR(nextval('work_seq'), 'fm00000000000000');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works DROP COLUMN work_number;
DROP SEQUENCE work_seq;
-- +goose StatementEnd
