-- +goose Up
-- +goose StatementBegin
ALTER TABLE variable_storage ADD COLUMN status TEXT NOT NULL DEFAULT 'idle';
UPDATE variable_storage SET status = 'finished' WHERE is_finished = TRUE;
ALTER TABLE variable_storage DROP COLUMN is_finished;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE variable_storage ADD COLUMN is_finished BOOL NOT NULL DEFAULT FALSE;
UPDATE variable_storage SET is_finished = TRUE WHERE status = 'finished';
ALTER TABLE variable_storage DROP COLUMN status;
-- +goose StatementEnd
