-- +goose Up
-- +goose StatementBegin
create table if not exists version_sla
(
    id uuid not null primary key ,
    version_id uuid not null,
    author varchar not null default '',
    created_at timestamp with time zone not null,
    work_type varchar not null default '',
    sla integer not null default 0
);
insert into version_sla(id,version_id, author, created_at, work_type,sla)
select uuid_generate_v4() ,id,author,now(),'8/5',40
       from versions;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists version_sla;
-- +goose StatementEnd
