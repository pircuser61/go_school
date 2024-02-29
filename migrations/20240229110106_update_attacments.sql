-- +goose Up
-- SQL in this section is executed when the migration is applied

UPDATE variable_storage vs
SET attachments = coalesce((
    WITH data AS (
        SELECT id,
            jsonb_array_elements(content -> 'State' -> step_name -> 'additional_info') -> 'attachments' AS additional_info_attachments,
            jsonb_array_elements(content -> 'State' -> step_name -> 'approver_log') -> 'attachments'    AS approver_log_attachments,
            jsonb_array_elements(content -> 'State' -> step_name -> 'request_execution_info_logs') -> 'attachments' AS execution_info_logs_attachments,
            jsonb_array_elements(content -> 'State' -> step_name -> 'editing_app_log') -> 'attachments' AS editing_app_log_attachments
        FROM variable_storage
    ),
    block_attachments AS (
        SELECT
            id,
            SUM(coalesce(jsonb_array_length(NULLIF(additional_info_attachments, 'null')), 0)) AS a1,
            SUM(coalesce(jsonb_array_length(NULLIF(approver_log_attachments, 'null')), 0)) AS a2,
            SUM(coalesce(jsonb_array_length(NULLIF(execution_info_logs_attachments, 'null')), 0)) AS a3,
            SUM(coalesce(jsonb_array_length(NULLIF(editing_app_log_attachments, 'null')), 0)) AS a4
        FROM data
        WHERE additional_info_attachments IS NOT NULL
           OR approver_log_attachments IS NOT NULL
           OR editing_app_log_attachments IS NOT NULL
           OR execution_info_logs_attachments IS NOT NULL
        GROUP BY id
    )
   SELECT
       coalesce(a1, 0) +
       coalesce(a2, 0) +
       coalesce(a3, 0) +
       coalesce(a4, 0)
   FROM block_attachments
   WHERE block_attachments.id = vs.id
), 0)
WHERE vs.step_type IN ('approver', 'execution', 'sign');

UPDATE variable_storage vs
SET attachments = COALESCE((
   SELECT
       count(*)
   FROM variable_storage
            CROSS JOIN LATERAL regexp_matches(content -> 'State' -> step_name ->> 'application_body'::text,
       'file_id|external_link|attachment', 'ig')
   WHERE id = vs.id
   GROUP BY id
), 0)
WHERE vs.step_type IN('form', 'servicedesk_application');

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
UPDATE variable_storage SET attachments = 0;
