-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS version_approval_lists(
    id uuid not null primary key ,
    version_id uuid references versions(id),
    name varchar not null default '',
    steps character varying[] default '{}'::character varying[],
    context_mapping jsonb default '{}'::jsonb not null,
    forms_mapping jsonb default '{}'::jsonb not null,
    created_at timestamp with time zone not null default now(),
    deleted_at timestamp with time zone
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS version_approval_lists;
-- +goose StatementEnd
