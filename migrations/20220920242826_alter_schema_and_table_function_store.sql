-- +goose Up
-- +goose StatementBegin
DROP TABLE function_store.functions;

CREATE TABLE function_store.functions
(
    function_id uuid not null,
    name        varchar(512) not null,
    created_at  timestamp with time zone not null default now(),
    updated_at  timestamp with time zone,
    deleted_at  timestamp with time zone,
    CONSTRAINT functions_pkey PRIMARY KEY (function_id)
);

ALTER TABLE function_store.functions
    ADD UNIQUE (name);

CREATE TABLE function_store.versions
(
    version_id  uuid not null,
    function_id uuid not null references function_store.functions(function_id),
    description text  not null default ''::text,
    version     varchar(128) not null,
    tenants     varchar[] not null default array[]::varchar[],
    input       jsonb not null,
    output      jsonb not null,
    created_at  timestamp with time zone not null default now(),
    updated_at  timestamp with time zone,
    deleted_at  timestamp with time zone,
    CONSTRAINT versions_pkey PRIMARY KEY (version_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS function_store.versions;
DROP TABLE IF EXISTS function_store.functions;

DROP SCHEMA if EXISTS function_store;
-- +goose StatementEnd
