-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table pipeliner.tags
    add column if not exists color text not null default '';

alter table pipeliner.tags drop column author;

alter table pipeliner.tags
    add column if not exists author text not null default '';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table pipeliner.tags drop column color;
