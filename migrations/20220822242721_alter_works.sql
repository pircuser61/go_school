-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN active_blocks jsonb,
    ADD COLUMN skipped_blocks jsonb,
    ADD COLUMN notified_blocks jsonb,
    ADD COLUMN prev_update_status_blocks jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN IF EXISTS active_blocks,
    DROP COLUMN IF EXISTS skipped_blocks,
    DROP COLUMN IF EXISTS notified_blocks,
    DROP COLUMN IF EXISTS prev_update_status_blocks;
-- +goose StatementEnd
