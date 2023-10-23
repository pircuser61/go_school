-- +goose Up
-- +goose StatementBegin
alter table works
    add column if not exists human_status_comment text not null default '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table works
    drop column if exists human_status_comment;
-- +goose StatementEnd
