-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table variable_storage
    add column if not exists attachments integer not null default 0;

comment on column variable_storage.attachments is 'Количество вложений в заявке';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table variable_storage drop column if exists attachments;