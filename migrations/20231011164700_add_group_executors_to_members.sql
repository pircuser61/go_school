-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table members
    add column if not exists execution_group_member bool not null default false;


-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table members
    drop column if exists execution_group_member;