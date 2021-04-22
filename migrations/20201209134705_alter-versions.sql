-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table pipeliner.versions
    add column if not exists comment_rejected text not null default '';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table pipeliner.versions drop column comment_rejected;