-- +goose Up
-- +goose StatementBegin
drop table if exists versions_07092022;

create table versions_07092022
(
    id               uuid                     not null
        constraint versions_07092022_pk
            primary key,
    status           smallint                 not null,
    pipeline_id      uuid                     not null,
    created_at       timestamp with time zone not null,
    content          jsonb                    not null,
    author           varchar,
    approver         varchar,
    comment_rejected text    default ''::text not null,
    comment          text    default ''::text not null,
    deleted_at       timestamp with time zone,
    last_run_id      uuid,
    is_actual        boolean default false,
    updated_at       timestamp with time zone
);

alter table versions_07092022
    drop constraint if exists versions_07092022_ok;

drop index if exists versions_07092022_pipeline_id_index;

alter table versions_07092022
    owner to jocasta;

create index versions_07092022_pipeline_id_index
    on versions_07092022 (pipeline_id);

insert into versions_07092022
select * from versions;

create or replace function start_migration()
    returns void
    language plpgsql
as $function$
declare v_cursor record;
begin
    for v_cursor in
        select
            id,
            n.pipeline_id,
            blockName,
            currentNext,
            json_agg(newNextSingle)::jsonb as newNext,
            ('{pipeline,blocks,' || blockName || ',sockets}')::text[] as updatePath
        from (
                 select pipeline_id,
                        id,
                        currentNext,
                        blockName,
                        json_build_object('id', m.socket_id, 'title', m.title, 'nextBlockIds',
                                          m.nextBlockIds) as newNextSingle
                 from (
                          select l.socket_id                             as socket_id,
                                 case
                                     when l.socket_id = 'default' then 'Выход по умолчанию'
                                     when l.socket_id = 'approved' then 'Согласовано'
                                     when l.socket_id = 'rejected' then 'Отклонено'
                                     when l.socket_id = 'edit_app' then 'На доработку'
                                     when l.socket_id = 'req_add_info' then 'Необходима дополнительная информация'
                                     when l.socket_id = 'executed' then 'Исполнено'
                                     when l.socket_id = 'not_executed' then 'Не исполнено'
                                     when l.socket_id = 'true' then 'Да'
                                     when l.socket_id = 'false' then 'Нет'
                                     end                                 as title,
                                 jsonb_object_field(l.next, l.socket_id) as nextBlockIds,
                                 pipeline_id,
                                 id,
                                 blockName,
                                 next as currentNext
                          from (
                                   select id,
                                          pipeline_id,
                                          k.next,
                                          jsonb_object_keys(k.next) as socket_id,
                                          k.blockName
                                   from (
                                            select id,
                                                   pipeline_id,
                                                   fields -> 'next' as next,
                                                   j.blockName
                                            from (
                                                     select id,
                                                            pipeline_id,
                                                            keys as blockName,
                                                            jsonb_object_field(cont, keys) as fields
                                                     from (
                                                              select id,
                                                                     pipeline_id,
                                                                     content -> 'pipeline' #> '{blocks}'                    as cont,
                                                                     jsonb_object_keys(content -> 'pipeline' #> '{blocks}') as keys
                                                              from versions
                                                          )
                                                              as i)
                                                     as j
                                            where fields ->> 'next' <> '{}')
                                            as k)
                                   as l)
                          as m
             ) as n
        group by n.id, n.pipeline_id, blockName, currentNext
        loop
            update versions
            set content = jsonb_set(content, v_cursor.updatePath, v_cursor.newNext, true)
            WHERE id = v_cursor.id;
        end loop;
end
$function$;

select * from start_migration();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
update versions
set
    content = versions_07092022.content
from versions_07092022
WHERE versions_07092022.id = versions.id
-- +goose StatementEnd