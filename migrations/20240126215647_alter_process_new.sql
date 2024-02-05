-- +goose Up
-- +goose StatementBegin

SELECT cron.unschedule('mv-processes-new-cron');

DROP MATERIALIZED VIEW IF EXISTS processes_new;

CREATE MATERIALIZED VIEW IF NOT EXISTS processes_new
AS
SELECT
    w.id AS work_id,
    w.work_number AS application_id,
    p.name AS process_name,
    '' AS process_sla,
    vs.step_type,
    vs.status,
    (((v.content::json -> 'pipeline') -> 'blocks') -> vs.step_name) -> 'title' AS description,
    ( SELECT
          CASE
              WHEN vs.step_type = 'approver' AND vs.content::json -> 'State' -> vs.step_name ->> 'approvers' != 'null'
                  THEN array_to_string(ARRAY( SELECT json_object_keys(vs.content::json -> 'State' -> vs.step_name -> 'approvers') AS keys), ',')
              WHEN vs.step_type = 'execution' AND vs.content::json -> 'State' -> vs.step_name ->> 'executors' != 'null'
                  THEN array_to_string(ARRAY( SELECT json_object_keys(vs.content::json -> 'State' -> vs.step_name -> 'executors') AS keys), ',')
              END AS "case") AS people,
    ((((v.content::json -> 'pipeline') -> 'blocks') -> vs.step_name) -> 'params') -> 'sla' AS block_sla,
    vs."time" AS started_at,
    ( SELECT
          CASE
              WHEN vs.status = 'finished' OR vs.status = 'no_success' THEN vs.updated_at
              ELSE NULL::timestamp with time zone
END AS "case") AS finished_at,
    w.finished_at AS process_finished_at,
    w.human_status AS process_status,
    w.rate,
    w.rate_comment,
    vsla.work_type,
    vsla.sla,
    ( SELECT content->'pipeline'->'blocks'-> 'servicedesk_application_0' -> 'params' ->> 'blueprint_id'
            FROM versions
            WHERE id = w.version_id) AS template_uuid
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
         LEFT JOIN version_sla vsla ON vsla.id = w.version_sla_id
WHERE w.child_id IS NULL;


GRANT SELECT ON TABLE processes_new TO report;

GRANT SELECT ON TABLE processes_new TO bi;

COMMENT ON MATERIALIZED VIEW processes_new
    IS 'Витрина с запущенными процессами';

COMMENT ON COLUMN processes_new.work_id
    IS 'Id заявки';

COMMENT ON COLUMN processes_new.application_id
    IS 'Рабочий номер заявки.';

COMMENT ON COLUMN processes_new.process_name
    IS 'Название сценария, по которому запущен процесс.';

COMMENT ON COLUMN processes_new.process_sla
    IS 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

COMMENT ON COLUMN processes_new.step_type
    IS 'Тип текущего блока из процесса.';

COMMENT ON COLUMN processes_new.status
    IS 'Статус текущего блока.';

COMMENT ON COLUMN processes_new.description
    IS 'Описание текущего блока из процесса.';

COMMENT ON COLUMN processes_new.people
    IS 'Участники текущего блока из процесса.';

COMMENT ON COLUMN processes_new.block_sla
    IS 'SLA текущего блока из процесса.';

COMMENT ON COLUMN processes_new.started_at
    IS 'Время запуска заявки по процессу.';

COMMENT ON COLUMN processes_new.finished_at
    IS 'Время окончания работы процесса по заявке.';

COMMENT ON COLUMN processes_new.process_finished_at
    IS 'Время окончания работы процесса.';

COMMENT ON COLUMN processes_new.process_status
    IS 'Статус процесса.';

COMMENT ON COLUMN processes_new.template_uuid
    IS 'id шаблона servicedesk';

SELECT cron.schedule('mv-processes-new-cron', '0 5 * * *', 'REFRESH MATERIALIZED VIEW processes_new WITH DATA');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT cron.unschedule('mv-processes-new-cron');

DROP MATERIALIZED VIEW IF EXISTS processes_new;

CREATE MATERIALIZED VIEW IF NOT EXISTS processes_new
AS
SELECT
    w.id AS work_id,
    w.work_number AS application_id,
    p.name AS process_name,
    '' AS process_sla,
    vs.step_type,
    vs.status,
    (((v.content::json -> 'pipeline') -> 'blocks') -> vs.step_name) -> 'title' AS description,
    ( SELECT
          CASE
              WHEN vs.step_type = 'approver' AND vs.content::json -> 'State' -> vs.step_name ->> 'approvers' != 'null'
                  THEN array_to_string(ARRAY( SELECT json_object_keys(vs.content::json -> 'State' -> vs.step_name -> 'approvers') AS keys), ',')
              WHEN vs.step_type = 'execution' AND vs.content::json -> 'State' -> vs.step_name ->> 'executors' != 'null'
                  THEN array_to_string(ARRAY( SELECT json_object_keys(vs.content::json -> 'State' -> vs.step_name -> 'executors') AS keys), ',')
              END AS "case") AS people,
    ((((v.content::json -> 'pipeline') -> 'blocks') -> vs.step_name) -> 'params') -> 'sla' AS block_sla,
    vs."time" AS started_at,
    ( SELECT
          CASE
              WHEN vs.status = 'finished' OR vs.status = 'no_success' THEN vs.updated_at
              ELSE NULL::timestamp with time zone
END AS "case") AS finished_at,
    w.finished_at AS process_finished_at,
    w.human_status AS process_status,
    w.rate,
    w.rate_comment,
    vsla.work_type,
    vsla.sla,
    ( SELECT content->'pipeline'->'blocks'-> 'servicedesk_application_0' -> 'params' ->> 'blueprint_id'
            FROM versions
            WHERE id = w.version_id) AS template_uuid
FROM works w
         LEFT JOIN variable_storage vs ON vs.work_id = w.id
         LEFT JOIN versions v ON v.id = w.version_id
         LEFT JOIN pipelines p ON p.id = v.pipeline_id
         LEFT JOIN version_sla vsla ON vsla.version_id = w.version_id
WHERE w.child_id IS NULL;


GRANT SELECT ON TABLE processes_new TO report;

GRANT SELECT ON TABLE processes_new TO bi;

COMMENT ON MATERIALIZED VIEW processes_new
    IS 'Витрина с запущенными процессами';

COMMENT ON COLUMN processes_new.work_id
    IS 'Id заявки';

COMMENT ON COLUMN processes_new.application_id
    IS 'Рабочий номер заявки.';

COMMENT ON COLUMN processes_new.process_name
    IS 'Название сценария, по которому запущен процесс.';

COMMENT ON COLUMN processes_new.process_sla
    IS 'НЕ ИСПОЛЬЗУЕТСЯ. SLA процесса.';

COMMENT ON COLUMN processes_new.step_type
    IS 'Тип текущего блока из процесса.';

COMMENT ON COLUMN processes_new.status
    IS 'Статус текущего блока.';

COMMENT ON COLUMN processes_new.description
    IS 'Описание текущего блока из процесса.';

COMMENT ON COLUMN processes_new.people
    IS 'Участники текущего блока из процесса.';

COMMENT ON COLUMN processes_new.block_sla
    IS 'SLA текущего блока из процесса.';

COMMENT ON COLUMN processes_new.started_at
    IS 'Время запуска заявки по процессу.';

COMMENT ON COLUMN processes_new.finished_at
    IS 'Время окончания работы процесса по заявке.';

COMMENT ON COLUMN processes_new.process_finished_at
    IS 'Время окончания работы процесса.';

COMMENT ON COLUMN processes_new.process_status
    IS 'Статус процесса.';

COMMENT ON COLUMN processes_new.template_uuid
    IS 'id шаблона servicedesk';

SELECT cron.schedule('mv-processes-new-cron', '0 5 * * *', 'REFRESH MATERIALIZED VIEW processes_new WITH DATA');

-- +goose StatementEnd
