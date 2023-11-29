-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_action_type(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    step_names varchar[];
    s_name varchar;
BEGIN
    step_names = array(
        SELECT jsonb_object_keys(v2.content -> 'pipeline' -> 'blocks')
        FROM pipeliner.public.versions v2
        WHERE jsonb_typeof(v2.content -> 'pipeline' -> 'blocks') = 'object'
            AND v2.pipeline_id = input_v_ids
    );
    FOREACH s_name IN ARRAY step_names
        LOOP
            IF s_name LIKE 'approver_%' THEN
                UPDATE pipeliner.public.versions v1
                SET content = jsonb_set(content,
                    array['pipeline', 'blocks', s_name, 'sockets']::varchar[],
                    (
                        SELECT jsonb_agg(jsonb_set(
                            v3::jsonb,
                            '{actionType}',
                            CASE
                                WHEN v3 ->> 'actionType' = ''::text OR v3 -> 'actionType' = null THEN
                                    CASE
                                        WHEN v3 ->> 'id' = 'approve' THEN to_jsonb('primary'::text)
                                        WHEN v3 ->> 'id' = 'reject' THEN to_jsonb('secondary'::text)
                                        ELSE to_jsonb('other'::text)
                                    END
                                WHEN v3 ->> 'actionType' != ''::text THEN v3 -> 'actionType'
                            END,
                            false
                        ))
                        FROM jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> s_name -> 'sockets') v3
                    ),
                    false)
                WHERE v1.pipeline_id = input_v_ids
                    AND content -> 'pipeline' -> 'blocks' -> s_name -> 'sockets' IS NOT NULL;
            END IF;
        END LOOP;
END $function$;

SELECT set_action_type(v.pipeline_id)
FROM pipeliner.public.versions v;

UPDATE pipeliner.public.members v
    SET actions = replace(actions::text, 'approved:', 'approve:primary')::text array;

UPDATE pipeliner.public.members
    SET actions = replace(actions::text, 'rejected:', 'reject:secondary')::text array;

UPDATE pipeliner.public.members
    SET actions = replace(actions::text, 'send_edit:', 'approver_send_edit_app:other')::text array;

CREATE OR REPLACE FUNCTION set_action_type_varstore(input_v_ids uuid) RETURNS void
    language plpgsql
AS $function$
DECLARE
    step_names varchar[];
    s_name varchar;
BEGIN
    step_names = array(
        SELECT jsonb_object_keys(content -> 'State')
        FROM pipeliner.public.variable_storage
        WHERE jsonb_typeof(content -> 'State') = 'object'
            AND id = input_v_ids);
    FOREACH s_name IN ARRAY step_names
        LOOP
            IF s_name LIKE 'approver_%' THEN
                UPDATE pipeliner.public.variable_storage v1
                SET content = jsonb_set(content,
                    array['State', s_name, 'action_list']::varchar[],
                    (
                        SELECT jsonb_agg(jsonb_set(
                        v3::jsonb,
                        '{id}',
                         CASE
                             WHEN v3 ->> 'id' = 'approved' THEN to_jsonb('approve'::text)
                             WHEN v3 ->> 'id' = 'rejected' THEN to_jsonb('reject'::text)
                             WHEN v3 ->> 'id' = 'send_edit' THEN to_jsonb('approver_send_edit_app'::text)
                             ELSE v3 -> 'id'
                        END,
                        false
                        ))
                        FROM jsonb_array_elements(content -> 'State' -> s_name -> 'action_list') v3
                    ),
                    false)
                WHERE id = input_v_ids;

                UPDATE pipeliner.public.variable_storage v1
                SET content = jsonb_set(content,
                    array['State', s_name, 'action_list']::varchar[],
                    (
                        SELECT jsonb_agg(jsonb_set(
                            v3::jsonb,
                            '{type}',
                            CASE
                                WHEN v3 ->> 'type' = ''::text OR v3 -> 'type' = null THEN
                                    CASE
                                        WHEN v3 ->> 'id' = 'approve' THEN to_jsonb('primary'::text)
                                        WHEN v3 ->> 'id' = 'reject' THEN to_jsonb('secondary'::text)
                                        ELSE to_jsonb('other'::text)
                                    END
                                WHEN v3 ->> 'type' != ''::text THEN v3 -> 'type'
                            END,
                            false
                        ))
                        FROM jsonb_array_elements(content -> 'State' -> s_name -> 'action_list') v3
                    ),
                    false)
                WHERE id = input_v_ids;
            END IF;
        END LOOP;
END $function$;

SELECT set_action_type_varstore(vs1.id)
FROM pipeliner.public.variable_storage vs1
INNER JOIN (
    SELECT work_id, min(time) AS first_create_approve
    FROM pipeliner.public.variable_storage
    WHERE step_name = 'approver_0'
    GROUP BY work_id
) vs2
    ON vs1.work_id = vs2.work_id
        AND vs1.time >= vs2.first_create_approve;

DROP function IF EXISTS set_action_type(input_v_ids uuid);

DROP function IF EXISTS set_action_type_varstore(input_v_ids uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
