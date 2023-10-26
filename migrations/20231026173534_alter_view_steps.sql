-- +goose Up
-- +goose StatementBegin
SELECT cron.unschedule('mv-steps-cron');

DROP MATERIALIZED VIEW IF EXISTS steps;

CREATE MATERIALIZED VIEW IF NOT EXISTS steps
AS
SELECT vs.id AS step_id,
    vs.step_type AS step_type,
    vs.step_name AS step_name,
    vs.status AS step_status,
    vs.time AS time,
    vs.updated_at AS updated_at,
    vs.content::json -> 'State' -> vs.step_name ->> 'check_sla' AS check_sla,
    vs.content::json -> 'State' -> vs.step_name ->> 'sla' AS sla,
    w.id AS work_id,
   ( SELECT
         CASE
             WHEN vs.step_type = 'approver' AND vs.content::json -> 'State' -> vs.step_name ->> 'approvers' != 'null'
                 THEN array_to_string(ARRAY( SELECT json_object_keys(vs.content::json -> 'State' -> vs.step_name -> 'approvers') AS keys), ',')
             WHEN vs.step_type = 'execution' AND vs.content::json -> 'State' -> vs.step_name ->> 'executors' != 'null'
                 THEN array_to_string(ARRAY( SELECT json_object_keys(vs.content::json -> 'State' -> vs.step_name -> 'executors') AS keys), ',')
             END AS "case") AS people,
   ( SELECT content->'pipeline'->'blocks'-> vs.step_name ->> 'title'
            FROM versions
            WHERE id = w.version_id) AS short_title
FROM works w
JOIN variable_storage vs ON vs.work_id = w.id
WHERE w.child_id IS NULL;

COMMENT ON MATERIALIZED VIEW steps
    IS 'Информация по блокам внутри процесса';

GRANT SELECT ON TABLE steps TO report;

COMMENT ON COLUMN steps.step_id
    IS 'Уникальный идентификатор записи перехода блока.';

COMMENT ON COLUMN steps.step_type
    IS 'Тип текущего блока из процесса.';

COMMENT ON COLUMN steps.step_name
    IS 'Идентификатор текущего блока.';

COMMENT ON COLUMN steps.step_status
    IS 'Статус текущего блока.';

COMMENT ON COLUMN steps.time
    IS 'Время создания записи';

COMMENT ON COLUMN steps.updated_at
    IS 'Дата последнего обновления записи в таблице.';

COMMENT ON COLUMN steps.check_sla
    IS 'Проверять SLA';

COMMENT ON COLUMN steps.sla
    IS 'Отведённое время на выполнение';

COMMENT ON COLUMN steps.work_id
    IS 'Id заявки';

COMMENT ON COLUMN steps.people
    IS 'Участники текущего блока из процесса.';

COMMENT ON COLUMN steps.short_title
    IS 'Краткое название блока';

GRANT SELECT ON TABLE steps TO report;

GRANT SELECT ON TABLE steps TO bi;

SELECT cron.schedule('mv-steps-cron', '0 5 * * *', 'REFRESH MATERIALIZED VIEW steps WITH DATA');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT cron.unschedule('mv-steps-cron');

DROP MATERIALIZED VIEW IF EXISTS steps;
-- +goose StatementEnd
