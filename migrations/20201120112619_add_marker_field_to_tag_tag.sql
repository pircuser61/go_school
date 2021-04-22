-- +goose Up
-- +goose StatementBegin
alter table pipeliner.tags
    add is_marker bool default false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table pipeliner.tags
    drop column is_marker;
-- +goose StatementEnd