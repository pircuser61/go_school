-- +goose Up
-- +goose StatementBegin
alter table edit_nodes_history
    add column if not exists created_at timestamp with time zone not null default now();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table edit_nodes_history
    drop column if exists created_at;
-- +goose StatementEnd
