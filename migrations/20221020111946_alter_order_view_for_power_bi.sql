-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS processes;

CREATE VIEW processes
AS
SELECT w.work_number application_id,
       p.name process_name,
       ''::text as process_sla,
       vs.step_type,
       vs.status,
       v.content::json->'pipeline'->'blocks'->step_name->'title' description,
       (SELECT
            CASE WHEN vs.step_type = 'approver'
                     THEN array_to_string(
                        array(SELECT
                            json_object_keys(vs.content::json -> 'State' -> step_name -> 'approvers'
                        ) AS keys),
                        ','
                    )
                 WHEN vs.step_type = 'execution'
                     THEN array_to_string(
                         array(SELECT
                              json_object_keys(vs.content::json -> 'State' -> step_name -> 'executors'
                        ) AS keys),
                         ','
                     )
                END
       ) people,
       vs.content::json->'State'->step_name->'sla' block_sla,
       vs.time as started_at,
       (SELECT
            CASE
                WHEN vs.status = 'finished' OR vs.status = 'no_success'
                    THEN vs.updated_at
                ELSE NULL
                END
       ) as finished_at,
       w.finished_at as process_finished_at,
       w.human_status process_status

FROM works w
         LEFT JOIN variable_storage vs on vs.work_id = w.id
         LEFT JOIN versions v on v.id = w.version_id
         LEFT JOIN pipelines p on p.id = v.pipeline_id
WHERE w.child_id IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS processes;

CREATE VIEW processes
AS
SELECT w.work_number application_id,
       p.name process_name,
       ''::text as process_sla,
       vs.step_type,
       vs.status,
       v.content::json->'pipeline'->'blocks'->step_name->'title' description,
       (SELECT
            CASE WHEN vs.step_type = 'approver'
                     THEN array_to_string(
                        array(SELECT
                            json_object_keys(vs.content::json -> 'State' -> step_name -> 'approvers'
                        ) AS keys),
                        ','
                    )
                 WHEN vs.step_type = 'execution'
                     THEN array_to_string(
                         array(SELECT
                              json_object_keys(vs.content::json -> 'State' -> step_name -> 'executors'
                        ) AS keys),
                         ','
                     )
                END
       ) people,
       vs.content::json->'State'->step_name->'sla' block_sla,
       vs.time as started_at,
       w.finished_at as process_fineshed_at,
       w.human_status process_status

FROM works w
         LEFT JOIN variable_storage vs on vs.work_id = w.id
         LEFT JOIN versions v on v.id = w.version_id
         LEFT JOIN pipelines p on p.id = v.pipeline_id
WHERE w.child_id IS NULL
ORDER BY vs.time;
-- +goose StatementEnd
