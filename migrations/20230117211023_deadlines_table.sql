-- +goose Up
-- +goose StatementBegin
create table deadlines (
    id uuid primary key,
    block_id uuid not null references variable_storage(id),
    deadline timestamp with time zone not null ,
    "action" text not null
);
CREATE INDEX ON deadlines(deadline);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE deadlines;
-- +goose StatementEnd
