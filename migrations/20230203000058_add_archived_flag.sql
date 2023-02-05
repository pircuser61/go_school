-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN archived boolean NOT NULL DEFAULT false;

CREATE INDEX works_archived_index
    on works (archived);

COMMENT ON COLUMN works.archived
    IS 'Флаг того, что заявка автоматически (по дедлайну) перенесена в архив.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS works_archived_index;

ALTER TABLE works
    DROP COLUMN IF EXISTS archived;
-- +goose StatementEnd
