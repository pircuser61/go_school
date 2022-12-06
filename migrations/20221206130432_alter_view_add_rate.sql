-- +goose Up
-- +goose StatementBegin
drop view if exists processes;

create view processes
            (application_id, process_name, process_sla, step_type, status, description, people, block_sla, started_at,
             finished_at, process_finished_at, process_status, rate, rate_comment)
as
SELECT w.work_number                                                              AS application_id,
       p.name                                                                     AS process_name,
       ''::text                                                                   AS process_sla,
       vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) ->
       'title'::text                                                              AS description,
       (SELECT CASE
                   WHEN vs.step_type::text = 'approver'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'approvers'::text) AS keys), ','::text)
                   WHEN vs.step_type::text = 'execution'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'executors'::text) AS keys), ','::text)
                   ELSE NULL::text
                   END AS "case")                                                 AS people,
       ((vs.content::json -> 'State'::text) -> vs.step_name::text) -> 'sla'::text AS block_sla,
       vs."time"                                                                  AS started_at,
       (SELECT CASE
                   WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
                   ELSE NULL::timestamp with time zone
                   END AS "case")                                                 AS finished_at,
       w.finished_at                                                              AS process_finished_at,
       w.human_status                                                             AS process_status,
       w.rate                                                                     as rate,
       w.rate_comment                                                             as rate_comment
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
WHERE w.child_id IS NULL;

alter table processes
    owner to jocasta;

grant select on processes to report;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop view if exists processes;

create view processes
            (application_id, process_name, process_sla, step_type, status, description, people, block_sla, started_at,
             finished_at, process_finished_at, process_status)
as
SELECT w.work_number                                                              AS application_id,
       p.name                                                                     AS process_name,
       ''::text                                                                   AS process_sla,
       vs.step_type,
       vs.status,
       (((v.content::json -> 'pipeline'::text) -> 'blocks'::text) -> vs.step_name::text) ->
       'title'::text                                                              AS description,
       (SELECT CASE
                   WHEN vs.step_type::text = 'approver'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'approvers'::text) AS keys), ','::text)
                   WHEN vs.step_type::text = 'execution'::text THEN array_to_string(
                           ARRAY(SELECT json_object_keys(((vs.content::json -> 'State'::text) -> vs.step_name::text) ->
                                                         'executors'::text) AS keys), ','::text)
                   ELSE NULL::text
                   END AS "case")                                                 AS people,
       ((vs.content::json -> 'State'::text) -> vs.step_name::text) -> 'sla'::text AS block_sla,
       vs."time"                                                                  AS started_at,
       (SELECT CASE
                   WHEN vs.status = 'finished'::text OR vs.status = 'no_success'::text THEN vs.updated_at
                   ELSE NULL::timestamp with time zone
                   END AS "case")                                                 AS finished_at,
       w.finished_at                                                              AS process_finished_at,
       w.human_status                                                             AS process_status
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
WHERE w.child_id IS NULL;

alter table processes
    owner to jocasta;

grant select on processes to report;
-- +goose StatementEnd
