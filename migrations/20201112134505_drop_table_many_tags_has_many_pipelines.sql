-- +goose Up
-- SQL in this section is executed when the migration is applied.
DROP TABLE IF EXISTS pipeliner.many_tags_has_many_pipelines;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
