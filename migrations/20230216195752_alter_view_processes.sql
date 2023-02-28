-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS processes;

CREATE VIEW processes
AS
SELECT w.work_number  AS application_id,
       p.name         AS process_name,
       ''::text       AS process_sla,
       vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) -> 'title'::text  AS description,
       (SELECT CASE
           WHEN vs.step_type::text = 'approver'::text THEN array_to_string(ARRAY(SELECT json_object_keys(
                ((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                'approvers'::text) AS keys), ','::text)
           WHEN vs.step_type::text = 'execution'::text THEN array_to_string(ARRAY(SELECT json_object_keys(
                 ((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                 'executors'::text) AS keys), ','::text)
           ELSE NULL::text
           END AS "case") AS people,
       (v.content::json -> 'pipeline' -> 'blocks' -> vs.step_name -> 'params' -> 'sla'::text) as block_sla,
       vs."time" AS started_at,
       (SELECT CASE
           WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
           ELSE NULL::timestamp with time zone
           END AS "case") AS finished_at,
       w.finished_at AS process_finished_at,
       w.human_status AS process_status,
       w.rate,
       w.rate_comment
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
WHERE w.child_id IS NULL;

comment on view processes is 'Витрина с запущенными процессами';

comment on column processes.application_id is 'Идентификатор заявки.';

comment on column processes.process_name is 'Название сценария, по которому запущен процесс.';

comment on column processes.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

comment on column processes.step_type is 'Тип текущего блока из процесса.';

comment on column processes.status is 'Статус текущего блока.';

comment on column processes.description is 'Описание текущего блока из процесса.';

comment on column processes.people is 'Участники текущего блока из процесса.';

comment on column processes.block_sla is 'SLA текущего блока из процесса.';

comment on column processes.started_at is 'Время запуска заявки по процессу.';

comment on column processes.finished_at is 'Время окончания работы процесса по заявке.';

comment on column processes.process_finished_at is 'Время окончания работы процесса.';

comment on column processes.process_status is 'Статус процесса.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS processes

CREATE VIEW processes
AS
SELECT w.work_number  AS application_id,
       p.name         AS process_name,
       ''::text       AS process_sla,
       vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) -> 'title'::text  AS description,
       (SELECT CASE
           WHEN vs.step_type::text = 'approver'::text THEN array_to_string(ARRAY(SELECT json_object_keys(
                ((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                'approvers'::text) AS keys), ','::text)
           WHEN vs.step_type::text = 'execution'::text THEN array_to_string(ARRAY(SELECT json_object_keys(
                 ((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                 'executors'::text) AS keys), ','::text)
           ELSE NULL::text
           END AS "case") AS people,
       ((vs.content::json -> 'State'::text) -> vs.step_name::text) -> 'sla'::text AS block_sla,
       vs."time" AS started_at,
       (SELECT CASE
           WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
           ELSE NULL::timestamp with time zone
           END AS "case") AS finished_at,
       w.finished_at AS process_finished_at,
       w.human_status AS process_status,
       w.rate,
       w.rate_comment
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
WHERE w.child_id IS NULL;

comment on view processes is 'Витрина с запущенными процессами';

comment on column processes.application_id is 'Идентификатор заявки.';

comment on column processes.process_name is 'Название сценария, по которому запущен процесс.';

comment on column processes.process_sla is 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

comment on column processes.step_type is 'Тип текущего блока из процесса.';

comment on column processes.status is 'Статус текущего блока.';

comment on column processes.description is 'Описание текущего блока из процесса.';

comment on column processes.people is 'Участники текущего блока из процесса.';

comment on column processes.block_sla is 'SLA текущего блока из процесса.';

comment on column processes.started_at is 'Время запуска заявки по процессу.';

comment on column processes.finished_at is 'Время окончания работы процесса по заявке.';

comment on column processes.process_finished_at is 'Время окончания работы процесса.';

comment on column processes.process_status is 'Статус процесса.';

-- +goose StatementEnd
