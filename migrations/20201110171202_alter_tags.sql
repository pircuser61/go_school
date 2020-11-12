-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table pipeliner.tags
    add column if not exists color smallint not null default 1;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table pipeliner.tags drop column color;