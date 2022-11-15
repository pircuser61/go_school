-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table versions
    add column if not exists deleted_at timestamp with time zone,
    add column if not exists last_run_id      uuid;

alter table pipelines
    add constraint uniq_name
        unique (name);

alter table variable_storage
    add constraint works_id
        foreign key (work_id) references works
            on update cascade on delete set null;

create index versions_pipeline_id_index
    on versions (pipeline_id);

alter table works
    add parent_work_id uuid;

alter table pipeline_history
    alter column approver type varchar using approver::varchar;

alter table pipeline_history
    alter column approver drop not null;

alter table pipelines
    alter column author type varchar using author::varchar;

alter table pipelines
    alter column author drop not null;

alter table versions
    alter column author type varchar using author::varchar;

alter table versions
    alter column author drop not null;

alter table versions
    alter column approver type varchar using approver::varchar;

alter table versions
    drop constraint pipelines_fk;

alter table versions
    drop column id_pipelines;

alter table versions
    drop constraint version_status_fk;

alter table versions
    drop column id_version_status;

alter table works
    alter column author type varchar using author::varchar;

alter table works
    drop constraint versions_fk;

alter table works
    drop column id_versions;

alter table works
    drop constraint work_status_fk;

alter table works
    drop column id_work_status;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table versions
    drop column if exists deleted_at,
    drop column if exists last_run_id;

alter table pipeline_history
    alter column approver type uuid using approver::uuid;

alter table pipeline_history
    alter column approver set not null;

alter table pipelines
    alter column author type uuid using author::uuid;

alter table pipelines
    alter column author set not null;

alter table pipelines
    drop constraint uniq_name;

alter table variable_storage
    drop constraint works_id;

alter table versions
    alter column author type uuid using author::uuid;

alter table versions
    alter column author set not null;

alter table versions
    alter column approver type uuid using approver::uuid;

drop index versions_pipeline_id_index;

alter table works
    alter column author type uuid using author::uuid;

alter table works
    drop column parent_work_id;

alter table versions
    add id_version_status smallint not null;

alter table versions
    add id_pipelines uuid not null;

alter table versions
    add constraint pipelines_fk
        foreign key (id_pipelines) references pipelines
            on update cascade on delete restrict;

alter table versions
    add constraint version_status_fk
        foreign key (id_version_status) references version_status
            on update cascade on delete restrict;

alter table works
    add id_versions uuid;

alter table works
    add id_work_status smallint;

alter table works
    add constraint versions_fk
        foreign key (id_versions) references versions
            on update cascade on delete set null;

alter table works
    add constraint work_status_fk
        foreign key (id_work_status) references work_status
            on update cascade on delete set null;
