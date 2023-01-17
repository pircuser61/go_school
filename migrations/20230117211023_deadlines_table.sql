-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
create table deadlines (
    id uuid primary key default gen_random_uuid(),
    block_id uuid not null references variable_storage(id),
    deadline timestamp with time zone not null ,
    "action" text not null
);
CREATE INDEX ON deadlines(deadline);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE deadlines;
DROP EXTENSION IF EXISTS "uuid-ossp"
-- +goose StatementEnd
