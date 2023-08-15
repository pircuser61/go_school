-- +goose Up
-- +goose StatementBegin
UPDATE pipeliner.public.works w
SET finished_at = (select max(updated_at) FROM pipeliner.public.variable_storage WHERE work_id = w.id)
WHERE status IN (2, 3, 4, 6)
  AND finished_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
