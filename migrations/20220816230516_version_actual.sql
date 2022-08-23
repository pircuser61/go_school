-- +goose Up
-- +goose StatementBegin
ALTER TABLE pipeliner.versions
    ADD COLUMN is_actual BOOLEAN DEFAULT FALSE;

UPDATE pipeliner.versions
SET is_actual = TRUE
WHERE id in (
    SELECT id
    FROM pipeliner.versions v1
    WHERE created_at = (
        SELECT MAX(created_at)
        FROM pipeliner.versions v2
        WHERE v2.pipeline_id = v1.pipeline_id
          AND v2.status = 2
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeliner.versions
    DROP COLUMN is_actual;
-- +goose StatementEnd
