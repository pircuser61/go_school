-- +goose Up
-- +goose StatementBegin
CREATE MATERIALIZED VIEW IF NOT EXISTS events AS
(with events_form as (select variable_storage.id                 as id,
                             jsonb_array_elements(variable_storage.content -> 'State' -> variable_storage.step_name ->
                                                  'changes_log') as jsonb_extract
                      from variable_storage
                      where step_type = 'form')
 select id                                               as step_id,
        events_form.jsonb_extract ->> 'executor'         as "user",
        null                                             as log_type,
        events_form.jsonb_extract ->> 'application_body' as event_body,
        events_form.jsonb_extract ->> 'created_at'       as created_at
 from events_form)
UNION
-- node approver
(with events_approver as (select variable_storage.id                        as id,
                                 jsonb_array_elements(
                                                     variable_storage.content -> 'State' ->
                                                     variable_storage.step_name ->
                                                     'approver_log')::jsonb as jsonb_extract_approver_log,
                                 null::jsonb                                as jsonb_extract_additional_info,
                                 null::jsonb                                as jsonb_extract_editing_app_log
                          from variable_storage
                          where step_type = 'approver'
                          union
                          select variable_storage.id                           as id,
                                 null::jsonb                                   as jsonb_extract_approver_log,
                                 jsonb_array_elements(
                                                     variable_storage.content -> 'State' ->
                                                     variable_storage.step_name ->
                                                     'additional_info')::jsonb as jsonb_extract_additional_info,
                                 null::jsonb                                   as jsonb_extract_editing_app_log
                          from variable_storage
                          where step_type = 'approver'
                          union
                          select variable_storage.id                           as id,
                                 null::jsonb                                   as jsonb_extract_approver_log,
                                 null::jsonb                                   as jsonb_extract_additional_info,
                                 jsonb_array_elements(
                                                     variable_storage.content -> 'State' ->
                                                     variable_storage.step_name ->
                                                     'editing_app_log')::jsonb as jsonb_extract_editing_app_log
                          from variable_storage
                          where step_type = 'approver'),
      events_approver_as_jsonb as (select (case
                                               when (events_approver.jsonb_extract_approver_log ->> 'log_type') = 'decision'
                                                   then jsonb_build_object(
                                                       'step_id',
                                                       events_approver.id,
                                                       'user',
                                                       events_approver.jsonb_extract_approver_log ->> 'login',
                                                       'log_type',
                                                       events_approver.jsonb_extract_approver_log ->> 'log_type',
                                                       'event_body',
                                                       events_approver.jsonb_extract_approver_log ->> 'decision',
                                                       'created_at',
                                                       events_approver.jsonb_extract_approver_log ->> 'created_at'
                                                   ) -- first point
                                               when (events_approver.jsonb_extract_approver_log ->> 'log_type') =
                                                    'addApprover'
                                                   then jsonb_build_object(
                                                       'step_id',
                                                       events_approver.id,
                                                       'user',
                                                       events_approver.jsonb_extract_approver_log ->> 'login',
                                                       'log_type',
                                                       events_approver.jsonb_extract_approver_log ->> 'log_type',
                                                       'event_body',
                                                       null,
                                                       'created_at',
                                                       events_approver.jsonb_extract_approver_log ->> 'created_at'
                                                   ) -- second point
                                               when (events_approver.jsonb_extract_approver_log ->> 'log_type') =
                                                    'additionalApproverDecision'
                                                   then jsonb_build_object(
                                                       'step_id',
                                                       events_approver.id,
                                                       'user',
                                                       events_approver.jsonb_extract_approver_log ->>
                                                       'login', -- TODO ask how to get user
                                                       'log_type',
                                                       events_approver.jsonb_extract_approver_log ->> 'log_type',
                                                       'event_body',
                                                       events_approver.jsonb_extract_approver_log ->>
                                                       'log_type', -- TODO is this right?
                                                       'created_at',
                                                       events_approver.jsonb_extract_approver_log ->>
                                                       'created_at' -- TODO ask how to get it
                                                   ) -- third point
                                               when (events_approver.jsonb_extract_additional_info ->> 'type') = 'reply'
                                                   then jsonb_build_object(
                                                       'id', events_approver.id,
                                                       'user',
                                                       events_approver.jsonb_extract_additional_info ->> 'login',
                                                       'log_type',
                                                       (events_approver.jsonb_extract_additional_info ->> 'type'),
                                                       'event_body',
                                                       null,
                                                       'created_at',
                                                       (events_approver.jsonb_extract_additional_info ->> 'created_at')
                                                   ) -- eighth point
                                               when (events_approver.jsonb_extract_additional_info ->> 'type') = 'request'
                                                   then jsonb_build_object(
                                                       'id', events_approver.id,
                                                       'user',
                                                       events_approver.jsonb_extract_additional_info ->> 'login',
                                                       'log_type',
                                                       (events_approver.jsonb_extract_additional_info ->> 'type'),
                                                       'event_body',
                                                       null,
                                                       'created_at',
                                                       (events_approver.jsonb_extract_additional_info ->> 'created_at')
                                                   ) -- seventh point
                                               when (events_approver.jsonb_extract_editing_app_log is not null)
                                                   then
                                                   jsonb_build_object(
                                                           'id', events_approver.id,
                                                           'user',
                                                           events_approver.jsonb_extract_editing_app_log ->> 'approver',
                                                           'log_type', 'editing_app',
                                                           'event_body', null,
                                                           'created_at',
                                                           events_approver.jsonb_extract_editing_app_log ->>
                                                           'created_at'
                                                       ) -- fourth and fifth point
          end) jsonb_object
                                   from events_approver)
 select (events_approver_as_jsonb.jsonb_object ->> 'step_id')::uuid step_id,
        (events_approver_as_jsonb.jsonb_object ->> 'user')          "user",
        (events_approver_as_jsonb.jsonb_object ->> 'log_type')      log_type,
        (events_approver_as_jsonb.jsonb_object ->> 'event_body')    event_body,
        (events_approver_as_jsonb.jsonb_object ->> 'created_at')    created_at
 from events_approver_as_jsonb
 where events_approver_as_jsonb.jsonb_object -> 'step_id' is not null
 union
 (select events_approver.id                                        step_id,
         events_approver.jsonb_extract_approver_log ->> 'login'    "user",
         events_approver.jsonb_extract_approver_log ->> 'log_type' log_type,
         events_approver.jsonb_extract_approver_log ->>
         'log_type'                                                event_body,
         events_approver.jsonb_extract_approver_log ->>
         'created_at'                                              created_at
  from events_approver
  where events_approver.jsonb_extract_approver_log ->> 'log_type' =
        'additionalApproverDecision') -- sixth point, TODO check if this right coz mb not right fields
)
UNION
-- node execution
(WITH events_execution AS (SELECT variable_storage.id                                               AS id,
                                  variable_storage.content -> 'State' -> variable_storage.step_name AS jsonb_extract,
                                  variable_storage.updated_at                                       AS updated_at
                           FROM variable_storage
                           WHERE step_type = 'execution')

 SELECT -- first point
        id                                                               AS step_id,
        jsonb_object_keys(events_execution.jsonb_extract -> 'executors') AS "user",
        'is_taken_in_work'                                               AS log_type,
        NULL                                                             AS event_body,
        (events_execution.jsonb_extract -> 'taken_in_work_at')::text     AS created_at
--     events_execution.jsonb_extract
 FROM events_execution
 WHERE events_execution.jsonb_extract ->> 'executors' != 'null'
   AND array_length(ARRAY(SELECT jsonb_object_keys(events_execution.jsonb_extract -> 'executors')), 1) = 1
   AND (events_execution.jsonb_extract -> 'is_taken_in_work')::boolean = true
 UNION
 SELECT -- second/sixth point
        id                                                               AS step_id,
        jsonb_object_keys(events_execution.jsonb_extract -> 'executors') AS "user",
        'decision'                                                       AS log_type,
        events_execution.jsonb_extract ->> 'decision'                    AS event_body,
        (SELECT CASE
                    WHEN events_execution.jsonb_extract -> 'decision_made_at' IS NULL
                        THEN events_execution.jsonb_extract ->> 'decision_made_at'
                    ELSE updated_at::text
                    END AS "case")                                       AS created_at
--     events_execution.jsonb_extract
 FROM events_execution
 WHERE events_execution.jsonb_extract ->> 'executors' != 'null'
   AND array_length(ARRAY(SELECT jsonb_object_keys(events_execution.jsonb_extract -> 'executors')), 1) = 1
   AND events_execution.jsonb_extract -> 'decision' IS NOT NULL
 UNION
 SELECT -- third point
        id                                                                                               AS step_id,
        jsonb_array_elements(events_execution.jsonb_extract -> 'change_executors_logs') ->> 'old_login'  AS "user",
        'change_executors_logs'                                                                          AS log_type,
        jsonb_array_elements(events_execution.jsonb_extract -> 'change_executors_logs') ->> 'new_login'  AS event_body,
        jsonb_array_elements(events_execution.jsonb_extract -> 'change_executors_logs') ->> 'created_at' AS created_at
 FROM events_execution
 UNION
 SELECT -- fourth point
        id           AS step_id,
        jsonb_array_elements(events_execution.jsonb_extract -> 'request_execution_info_logs') ->>
        'login'      AS "user",
        jsonb_array_elements(events_execution.jsonb_extract -> 'request_execution_info_logs') ->>
        'req_type'   AS log_type,
        NULL         AS event_body,
        jsonb_array_elements(events_execution.jsonb_extract -> 'request_execution_info_logs') ->>
        'created_at' AS created_at
 FROM events_execution
 UNION
 SELECT -- fifth point
        id                                                                                         AS step_id,
        jsonb_array_elements(events_execution.jsonb_extract -> 'editing_app_log') ->> 'executor'   AS "user",
        'editing_app_log'                                                                          AS log_type,
        NULL                                                                                       AS event_body,
        jsonb_array_elements(events_execution.jsonb_extract -> 'editing_app_log') ->> 'created_at' AS created_at
 FROM events_execution);
SELECT cron.schedule('mv-events-cron', '0 5 * * *', 'REFRESH MATERIALIZED VIEW events WITH DATA');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP MATERIALIZED VIEW IF EXISTS events;
SELECT cron.unschedule('mv-events-cron');
-- +goose StatementEnd
