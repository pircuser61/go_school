-- +goose Up
-- SQL in this section is executed when the migration is applied

WITH blocks_with_filtered_forms AS (
    SELECT work_id, value(blocks) AS blocks
    FROM (
        SELECT work_id, jsonb_each(state) AS blocks
        FROM works w
        JOIN LATERAL (
            SELECT work_id, content::jsonb->'State' AS state
            FROM variable_storage vs
            WHERE vs.work_id = w.id
            ORDER BY vs.time DESC LIMIT 1
        ) descr ON descr.work_id = w.id
        WHERE state IS NOT NULL
    ) blocks_with_work_id
    WHERE key(blocks) NOT LIKE 'form%%'
    ),
    data AS (
        SELECT work_id,
        value(jsonb_each(blocks -> 'application_body'))					       AS form_and_sd_application_body,
            jsonb_array_elements(blocks -> 'additional_info') -> 'attachments' AS additional_info_attachments,
            jsonb_array_elements(blocks -> 'approver_log') -> 'attachments'    AS approver_log_attachments,
            jsonb_array_elements(blocks -> 'editing_app_log') -> 'attachments' AS editing_app_log_attachments
        FROM blocks_with_filtered_forms
        WHERE jsonb_typeof(blocks -> 'application_body') = 'object'
    ),
    counts AS (
         SELECT
             work_id,
             SUM(
                CASE
                 WHEN jsonb_typeof(form_and_sd_application_body) = 'object'
                     THEN 1
                 WHEN jsonb_typeof(form_and_sd_application_body) = 'array'
                     THEN jsonb_array_length(form_and_sd_application_body)
                 ELSE 0
                 END
             ) AS form_and_sd_count,
             SUM(coalesce(jsonb_array_length(NULLIF(additional_info_attachments, 'null')), 0)) AS attach_count,
             SUM(coalesce(jsonb_array_length(NULLIF(approver_log_attachments, 'null')), 0)) AS additional_approvers_count,
             SUM(coalesce(jsonb_array_length(NULLIF(editing_app_log_attachments, 'null')), 0)) AS rework_count
         FROM data
         WHERE form_and_sd_application_body::text LIKE '{"file_id":%%'
            OR form_and_sd_application_body::text LIKE '[{"file_id":%%'
            OR form_and_sd_application_body::text LIKE '{"external_link":%%'
            OR form_and_sd_application_body::text LIKE '[{"external_link":%%'
            OR form_and_sd_application_body::text LIKE '"attachment:%%'
            OR form_and_sd_application_body::text LIKE '["attachment:%%'
            OR additional_info_attachments IS NOT NULL
            OR approver_log_attachments IS NOT NULL
            OR editing_app_log_attachments IS NOT NULL
         GROUP BY work_id
    )
UPDATE variable_storage
SET attachments = coalesce((
   SELECT
       coalesce(form_and_sd_count, 0) +
       coalesce(attach_count, 0) +
       coalesce(additional_approvers_count, 0) +
       coalesce(rework_count, 0)
   FROM counts
   WHERE counts.work_id = variable_storage.work_id
), 0)
WHERE step_type = 'start';

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
UPDATE variable_storage SET attachments = 0;
