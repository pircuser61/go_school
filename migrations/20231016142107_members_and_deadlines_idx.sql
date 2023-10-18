-- +goose Up
-- +goose StatementBegin
CREATE INDEX deadlines_block_id_idx ON pipeliner.public.deadlines (block_id);
CREATE INDEX members_block_id_idx ON pipeliner.public.members (block_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP index members_block_id_idx;
DROP index deadlines_block_id_idx;
-- +goose StatementEnd
