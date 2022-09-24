-- +goose Up
-- +goose StatementBegin
create table if not exists pipeliner.cache_time_update (
    update_time timestamp with time zone
);

insert into pipeliner.cache_time_update(update_time) values('1000-01-01'::timestamp)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists pipeliner.cache_time_update
-- +goose StatementEnd