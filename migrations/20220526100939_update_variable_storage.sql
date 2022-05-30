-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table pipeliner.variable_storage
    add column if not exists step_type varchar(32) not null default '',
    add column if not exists is_finished bool not null default false;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table pipeliner.variable_storage
    drop column if exists step_type,
    drop column if exists is_finished;
