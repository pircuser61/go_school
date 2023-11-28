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
                            CASE WHEN v3 ->> 'actionType' = ''::text OR v3 -> 'actionType' = null THEN
                                CASE
                                    WHEN v3 ->> 'id' = 'approve' THEN to_jsonb('primary'::text)
                                    WHEN v3 ->> 'id' = 'reject' THEN to_jsonb('secondary'::text)
                                    ELSE to_jsonb('other'::text)
                                END
                            END,
                            true
                        ))
                        FROM jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> s_name -> 'sockets') v3
                    ),
                    true)
                WHERE v1.pipeline_id = input_v_ids
                    AND content -> 'pipeline' -> 'blocks' -> s_name -> 'sockets' IS NOT NULL;
            END IF;
        END LOOP;
END $function$;

SELECT set_action_type(v.pipeline_id)
FROM pipeliner.public.versions v;

UPDATE pipeliner.public.members v
    SET actions = replace(actions::text, 'approved', 'approve')::text array;

UPDATE pipeliner.public.members
    SET actions = replace(actions::text, 'rejected', 'reject')::text array;

UPDATE pipeliner.public.members
    SET actions = replace(actions::text, 'send_edit', 'approver_send_edit_app')::text array;

CREATE OR REPLACE FUNCTION set_action_type() RETURNS void
    language plpgsql
AS $function$
DECLARE
    step_names varchar[];
    s_name varchar;
BEGIN
    step_names = array(
            SELECT jsonb_object_keys(content -> 'State')
            FROM pipeliner.public.variable_storage
            WHERE jsonb_typeof(content -> 'State') = 'object');
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
                                     END
                                true
                                         ))
                        FROM jsonb_array_elements(content -> 'State' -> s_name -> 'action_list') v3
                    ),
                    true);

                UPDATE pipeliner.public.variable_storage v1
                SET content = jsonb_set(content,
                    array['State', s_name, 'action_list']::varchar[],
                    (
                        SELECT jsonb_agg(jsonb_set(
                            v3::jsonb,
                            '{type}',
                            CASE WHEN v3 ->> 'type' = ''::text OR v3 -> 'type' = null THEN
                                 CASE
                                     WHEN v3 ->> 'id' = 'approve' THEN to_jsonb('primary'::text)
                                     WHEN v3 ->> 'id' = 'reject' THEN to_jsonb('secondary'::text)
                                     ELSE to_jsonb('other'::text)
                                 END
                            END,
                            true
                        ))
                        FROM jsonb_array_elements(content -> 'State' -> s_name -> 'action_list') v3
                    ),
                    true);
            END IF;
        END LOOP;
END $function$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
