-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS edit_nodes_history(
    id uuid not null primary key ,
    event_id uuid not null,
    step_id uuid not null,
    time timestamp with time zone not null default now(),
    content jsonb NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS edit_nodes_history;
-- +goose StatementEnd
