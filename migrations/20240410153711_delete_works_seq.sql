-- +goose Up
-- +goose StatementBegin
SELECT cron.unschedule('mv-processes-new-cron');
DROP MATERIALIZED VIEW IF EXISTS processes_new;
DROP VIEW IF EXISTS processes;

ALTER TABLE works
ALTER COLUMN work_number TYPE text, ALTER work_number SET NOT NULL;

create view public.processes
            (application_id, process_name, process_sla, step_type, status, description, people, block_sla, started_at,
             finished_at, process_finished_at, process_status, rate, rate_comment)
as
SELECT w.work_number                                                                                      AS application_id,
       p.name                                                                                             AS process_name,
       ''::text                                                                                           AS process_sla,
        vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) ->
       'title'::text                                                                                      AS description,
       (SELECT CASE
                   WHEN vs.step_type::text = 'approver'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'approvers'::text) AS keys), ','::text)
                   WHEN vs.step_type::text = 'execution'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'executors'::text) AS keys), ','::text)
                   ELSE NULL::text
                   END AS "case")                                                                         AS people,
       ((((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) -> 'params'::text) ->
       'sla'::text                                                                                        AS block_sla,
       vs."time"                                                                                          AS started_at,
       (SELECT CASE
                   WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
                   ELSE NULL::timestamp with time zone
                   END AS "case")                                                                         AS finished_at,
       w.finished_at                                                                                      AS process_finished_at,
       w.human_status                                                                                     AS process_status,
       w.rate,
       w.rate_comment
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
WHERE w.child_id IS NULL;

comment on view public.processes is 'Витрина с запущенными процессами';

comment on column public.processes.application_id is 'Идентификатор заявки.';

comment on column public.processes.process_name is 'Название сценария, по которому запущен процесс.';

comment on column public.processes.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

comment on column public.processes.step_type is 'Тип текущего блока из процесса.';

comment on column public.processes.status is 'Статус текущего блока.';

comment on column public.processes.description is 'Описание текущего блока из процесса.';

comment on column public.processes.people is 'Участники текущего блока из процесса.';

comment on column public.processes.block_sla is 'SLA текущего блока из процесса.';

comment on column public.processes.started_at is 'Время запуска заявки по процессу.';

comment on column public.processes.finished_at is 'Время окончания работы процесса по заявке.';

comment on column public.processes.process_finished_at is 'Время окончания работы процесса.';

comment on column public.processes.process_status is 'Статус процесса.';

alter table public.processes
    owner to jocasta;

grant select on public.processes to report;

create materialized view public.processes_new as
SELECT w.id                                                                                               AS work_id,
       w.work_number                                                                                      AS application_id,
       p.name                                                                                             AS process_name,
       ''::text                                                                                           AS process_sla,
        vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) ->
       'title'::text                                                                                      AS description,
       (SELECT CASE
    WHEN vs.step_type::text = 'approver'::text AND
    (((vs.content::json -> 'State'::text) -> vs.step_name::text) ->> 'approvers'::text) <> 'null'::text
    THEN array_to_string(
    ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
    'approvers'::text) AS keys), ','::text)
    WHEN vs.step_type::text = 'execution'::text AND
    (((vs.content::json -> 'State'::text) -> vs.step_name::text) ->> 'executors'::text) <> 'null'::text
    THEN array_to_string(
    ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
    'executors'::text) AS keys), ','::text)
    ELSE NULL::text
    END AS "case")                                                                         AS people,
    ((((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) -> 'params'::text) ->
    'sla'::text                                                                                        AS block_sla,
    vs."time"                                                                                          AS started_at,
    (SELECT CASE
    WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
    ELSE NULL::timestamp with time zone
    END AS "case")                                                                         AS finished_at,
    w.finished_at                                                                                      AS process_finished_at,
    w.human_status                                                                                     AS process_status,
    w.rate,
    w.rate_comment,
    vsla.work_type,
    vsla.sla,
    (SELECT ((((versions.content -> 'pipeline'::text) -> 'blocks'::text) -> 'servicedesk_application_0'::text) ->
    'params'::text) ->> 'blueprint_id'::text
FROM versions
WHERE versions.id = w.version_id)                                                                 AS template_uuid
FROM works w
    LEFT JOIN variable_storage vs ON vs.work_id = w.id
    LEFT JOIN versions v ON v.id = w.version_id
    LEFT JOIN pipelines p ON p.id = v.pipeline_id
    LEFT JOIN version_sla vsla ON vsla.id = w.version_sla_id
WHERE w.child_id IS NULL;

comment on materialized view public.processes_new is 'Витрина с запущенными процессами';

comment on column public.processes_new.work_id is 'Id заявки';

comment on column public.processes_new.application_id is 'Рабочий номер заявки.';

comment on column public.processes_new.process_name is 'Название сценария, по которому запущен процесс.';

comment on column public.processes_new.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

comment on column public.processes_new.step_type is 'Тип текущего блока из процесса.';

comment on column public.processes_new.status is 'Статус текущего блока.';

comment on column public.processes_new.description is 'Описание текущего блока из процесса.';

comment on column public.processes_new.people is 'Участники текущего блока из процесса.';

comment on column public.processes_new.block_sla is 'SLA текущего блока из процесса.';

comment on column public.processes_new.started_at is 'Время запуска заявки по процессу.';

comment on column public.processes_new.finished_at is 'Время окончания работы процесса по заявке.';

comment on column public.processes_new.process_finished_at is 'Время окончания работы процесса.';

comment on column public.processes_new.process_status is 'Статус процесса.';

comment on column public.processes_new.template_uuid is 'id шаблона servicedesk';

alter materialized view public.processes_new owner to jocasta;

grant select on public.processes_new to report;

grant select on public.processes_new to bi;

SELECT cron.schedule('mv-processes-new-cron', '0 5 * * *', 'REFRESH MATERIALIZED VIEW processes_new WITH DATA');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT cron.unschedule('mv-processes-new-cron');
DROP MATERIALIZED VIEW IF EXISTS processes_new;
DROP VIEW IF EXISTS processes;

ALTER TABLE works
ALTER COLUMN work_number TYPE text default ('J'::text ||
         to_char(nextval('work_seq'::regclass), 'fm00000000000000'::text)), ALTER work_number SET NOT NULL;


create materialized view public.processes_new as
SELECT w.id                                                                                               AS work_id,
       w.work_number                                                                                      AS application_id,
       p.name                                                                                             AS process_name,
       ''::text                                                                                           AS process_sla,
        vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) ->
       'title'::text                                                                                      AS description,
       (SELECT CASE
    WHEN vs.step_type::text = 'approver'::text AND
    (((vs.content::json -> 'State'::text) -> vs.step_name::text) ->> 'approvers'::text) <> 'null'::text
    THEN array_to_string(
    ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
    'approvers'::text) AS keys), ','::text)
    WHEN vs.step_type::text = 'execution'::text AND
    (((vs.content::json -> 'State'::text) -> vs.step_name::text) ->> 'executors'::text) <> 'null'::text
    THEN array_to_string(
    ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
    'executors'::text) AS keys), ','::text)
    ELSE NULL::text
    END AS "case")                                                                         AS people,
    ((((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) -> 'params'::text) ->
    'sla'::text                                                                                        AS block_sla,
    vs."time"                                                                                          AS started_at,
    (SELECT CASE
    WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
    ELSE NULL::timestamp with time zone
    END AS "case")                                                                         AS finished_at,
    w.finished_at                                                                                      AS process_finished_at,
    w.human_status                                                                                     AS process_status,
    w.rate,
    w.rate_comment,
    vsla.work_type,
    vsla.sla,
    (SELECT ((((versions.content -> 'pipeline'::text) -> 'blocks'::text) -> 'servicedesk_application_0'::text) ->
    'params'::text) ->> 'blueprint_id'::text
FROM versions
WHERE versions.id = w.version_id)                                                                 AS template_uuid
FROM works w
    LEFT JOIN variable_storage vs ON vs.work_id = w.id
    LEFT JOIN versions v ON v.id = w.version_id
    LEFT JOIN pipelines p ON p.id = v.pipeline_id
    LEFT JOIN version_sla vsla ON vsla.id = w.version_sla_id
WHERE w.child_id IS NULL;

comment on materialized view public.processes_new is 'Витрина с запущенными процессами';

comment on column public.processes_new.work_id is 'Id заявки';

comment on column public.processes_new.application_id is 'Рабочий номер заявки.';

comment on column public.processes_new.process_name is 'Название сценария, по которому запущен процесс.';

comment on column public.processes_new.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

comment on column public.processes_new.step_type is 'Тип текущего блока из процесса.';

comment on column public.processes_new.status is 'Статус текущего блока.';

comment on column public.processes_new.description is 'Описание текущего блока из процесса.';

comment on column public.processes_new.people is 'Участники текущего блока из процесса.';

comment on column public.processes_new.block_sla is 'SLA текущего блока из процесса.';

comment on column public.processes_new.started_at is 'Время запуска заявки по процессу.';

comment on column public.processes_new.finished_at is 'Время окончания работы процесса по заявке.';

comment on column public.processes_new.process_finished_at is 'Время окончания работы процесса.';

comment on column public.processes_new.process_status is 'Статус процесса.';

comment on column public.processes_new.template_uuid is 'id шаблона servicedesk';

alter materialized view public.processes_new owner to jocasta;

grant select on public.processes_new to report;

grant select on public.processes_new to bi;

SELECT cron.schedule('mv-processes-new-cron', '0 5 * * *', 'REFRESH MATERIALIZED VIEW processes_new WITH DATA');

create view public.processes
            (application_id, process_name, process_sla, step_type, status, description, people, block_sla, started_at,
             finished_at, process_finished_at, process_status, rate, rate_comment)
as
SELECT w.work_number                                                                                      AS application_id,
       p.name                                                                                             AS process_name,
       ''::text                                                                                           AS process_sla,
        vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) ->
       'title'::text                                                                                      AS description,
       (SELECT CASE
                   WHEN vs.step_type::text = 'approver'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'approvers'::text) AS keys), ','::text)
                   WHEN vs.step_type::text = 'execution'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'executors'::text) AS keys), ','::text)
                   ELSE NULL::text
                   END AS "case")                                                                         AS people,
       ((((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) -> 'params'::text) ->
       'sla'::text                                                                                        AS block_sla,
       vs."time"                                                                                          AS started_at,
       (SELECT CASE
                   WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
                   ELSE NULL::timestamp with time zone
                   END AS "case")                                                                         AS finished_at,
       w.finished_at                                                                                      AS process_finished_at,
       w.human_status                                                                                     AS process_status,
       w.rate,
       w.rate_comment
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
WHERE w.child_id IS NULL;

comment on view public.processes is 'Витрина с запущенными процессами';

comment on column public.processes.application_id is 'Идентификатор заявки.';

comment on column public.processes.process_name is 'Название сценария, по которому запущен процесс.';

comment on column public.processes.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

comment on column public.processes.step_type is 'Тип текущего блока из процесса.';

comment on column public.processes.status is 'Статус текущего блока.';

comment on column public.processes.description is 'Описание текущего блока из процесса.';

comment on column public.processes.people is 'Участники текущего блока из процесса.';

comment on column public.processes.block_sla is 'SLA текущего блока из процесса.';

comment on column public.processes.started_at is 'Время запуска заявки по процессу.';

comment on column public.processes.finished_at is 'Время окончания работы процесса по заявке.';

comment on column public.processes.process_finished_at is 'Время окончания работы процесса.';

comment on column public.processes.process_status is 'Статус процесса.';

alter table public.processes
    owner to jocasta;

grant select on public.processes to report;
-- +goose StatementEnd
