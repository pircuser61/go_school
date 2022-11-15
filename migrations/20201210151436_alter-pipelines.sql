-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table versions
    add column if not exists comment text not null default '';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table versions drop column comment;