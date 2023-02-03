-- +goose Up
-- +goose StatementBegin
ALTER TABLE works
    ADD COLUMN archived boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN works.archived
    IS 'Флаг того, что заявка перенесена в архив.';

UPDATE works SET archived = true WHERE (now()::TIMESTAMP - finished_at::TIMESTAMP) > '3 days';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE works
    DROP COLUMN IF EXISTS archived;;
-- +goose StatementEnd
