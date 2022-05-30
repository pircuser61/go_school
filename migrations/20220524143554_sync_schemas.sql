-- +goose Up
-- SQL in this section is executed when the migration is applied.
alter table pipeliner.versions
    add column if not exists deleted_at timestamp with time zone,
    add column if not exists last_run_id      uuid;

alter table pipeliner.pipelines
    add constraint uniq_name
        unique (name);

alter table pipeliner.variable_storage
    add constraint works_id
        foreign key (work_id) references pipeliner.works
            on update cascade on delete set null;

create index versions_pipeline_id_index
    on pipeliner.versions (pipeline_id);

alter table pipeliner.works
    add parent_work_id uuid;

alter table pipeliner.pipeline_history
    alter column approver type varchar using approver::varchar;

alter table pipeliner.pipeline_history
    alter column approver drop not null;

alter table pipeliner.pipelines
    alter column author type varchar using author::varchar;

alter table pipeliner.pipelines
    alter column author drop not null;

alter table pipeliner.versions
    alter column author type varchar using author::varchar;

alter table pipeliner.versions
    alter column author drop not null;

alter table pipeliner.versions
    alter column approver type varchar using approver::varchar;

alter table pipeliner.versions
    drop constraint pipelines_fk;

alter table pipeliner.versions
    drop column id_pipelines;

alter table pipeliner.versions
    drop constraint version_status_fk;

alter table pipeliner.versions
    drop column id_version_status;

alter table pipeliner.works
    alter column author type varchar using author::varchar;

alter table pipeliner.works
    drop constraint versions_fk;

alter table pipeliner.works
    drop column id_versions;

alter table pipeliner.works
    drop constraint work_status_fk;

alter table pipeliner.works
    drop column id_work_status;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
alter table pipeliner.versions
    drop column if exists deleted_at,
    drop column if exists last_run_id;

alter table pipeliner.pipeline_history
    alter column approver type uuid using approver::uuid;

alter table pipeliner.pipeline_history
    alter column approver set not null;

alter table pipeliner.pipelines
    alter column author type uuid using author::uuid;

alter table pipeliner.pipelines
    alter column author set not null;

alter table pipeliner.pipelines
    drop constraint uniq_name;

alter table pipeliner.variable_storage
    drop constraint works_id;

alter table pipeliner.versions
    alter column author type uuid using author::uuid;

alter table pipeliner.versions
    alter column author set not null;

alter table pipeliner.versions
    alter column approver type uuid using approver::uuid;

drop index pipeliner.versions_pipeline_id_index;

alter table pipeliner.works
    alter column author type uuid using author::uuid;

alter table pipeliner.works
    drop column parent_work_id;

alter table pipeliner.versions
    add id_version_status smallint not null;

alter table pipeliner.versions
    add id_pipelines uuid not null;

alter table pipeliner.versions
    add constraint pipelines_fk
        foreign key (id_pipelines) references pipeliner.pipelines
            on update cascade on delete restrict;

alter table pipeliner.versions
    add constraint version_status_fk
        foreign key (id_version_status) references pipeliner.version_status
            on update cascade on delete restrict;

alter table pipeliner.works
    add id_versions uuid;

alter table pipeliner.works
    add id_work_status smallint;

alter table pipeliner.works
    add constraint versions_fk
        foreign key (id_versions) references pipeliner.versions
            on update cascade on delete set null;

alter table pipeliner.works
    add constraint work_status_fk
        foreign key (id_work_status) references pipeliner.work_status
            on update cascade on delete set null;
