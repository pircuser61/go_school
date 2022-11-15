-- +goose Up
-- +goose StatementBegin
ALTER TABLE versions
    ADD COLUMN is_actual BOOLEAN DEFAULT FALSE;

UPDATE versions
SET is_actual = TRUE
WHERE id in (
    SELECT id
    FROM versions v1
    WHERE created_at = (
        SELECT MAX(created_at)
        FROM versions v2
        WHERE v2.pipeline_id = v1.pipeline_id
          AND v2.status = 2
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE versions
    DROP COLUMN is_actual;
-- +goose StatementEnd
