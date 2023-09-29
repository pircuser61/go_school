-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table members
    add column if not exists is_acted bool not null default false;


-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table members
    drop column if exists is_acted;
