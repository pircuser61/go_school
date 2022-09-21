-- +goose Up
-- +goose StatementBegin
DROP TABLE function_store.functions;

CREATE TABLE function_store.functions
(
    id          uuid not null,
    name        varchar(512) not null,
    description text  not null default ''::text,
    version     varchar(128) not null,
    tenants     varchar[] not null default array[]::varchar[],
    input       jsonb not null,
    output      jsonb not null,
    created_at  timestamp with time zone not null default now(),
    updated_at  timestamp with time zone,
    CONSTRAINT functions_pkey PRIMARY KEY (id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS function_store.functions;
-- +goose StatementEnd
