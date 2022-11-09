-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table tags
    add column if not exists color text not null default '';

alter table tags drop column author;

alter table tags
    add column if not exists author text not null default '';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table tags drop column color;
