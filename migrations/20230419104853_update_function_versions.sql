-- +goose Up
-- +goose StatementBegin

-- !!! Необходимо подставить тэги в зависимости от стенда на котором запускается миграция !!! --

CREATE OR REPLACE FUNCTION update_function_versions()
    RETURNS void
    LANGUAGE plpgsql
AS $function$
DECLARE
    versions uuid[] := array(SELECT DISTINCT id FROM versions);
    step_names varchar[];
    v_id uuid;
    s_name varchar;
    summator_tag CONSTANT varchar := '"v1.0.9-alpha.1"';
    tms_caller_tag CONSTANT varchar := '"v1.3.5-alpha.1"';
    tms_attachment_tag CONSTANT varchar := '"v1.1.8-alpha.2"';
    hr_departmental_awards_tag CONSTANT varchar := '"v1.0.0-alpha.6"';
    kafka_producer_test_go_tag CONSTANT varchar := '"0.0.1-rc.2"';
    summator_go_tag CONSTANT varchar := '"v1.0.1-alpha.5"';
BEGIN
    FOREACH v_id IN ARRAY versions
        LOOP
            step_names = array(
                    SELECT jsonb_object_keys(content -> 'pipeline' -> 'blocks')
                    FROM versions
                    WHERE id = v_id AND deleted_at IS NULL AND
                            jsonb_typeof(content -> 'pipeline' -> 'blocks') = 'object'
                );

            FOREACH s_name IN ARRAY step_names
                LOOP
                    IF s_name LIKE 'executable_function%' THEN
                        UPDATE versions
                        SET content = jsonb_set(
                                jsonb_set(
                                        content,
                                        array['pipeline', 'blocks', s_name, 'params', 'version']::varchar[],
                                        summator_tag::jsonb, true
                                    ),
                                array['pipeline', 'blocks', s_name, 'params', 'function', 'version']::varchar[],
                                summator_tag::jsonb, true
                            )
                        WHERE id = v_id AND content -> 'pipeline' -> 'blocks' -> s_name -> 'params' ->> 'name' = 'summator';

                        UPDATE versions
                        SET content = jsonb_set(
                                jsonb_set(
                                        content,
                                        array['pipeline', 'blocks', s_name, 'params', 'version']::varchar[],
                                        tms_caller_tag::jsonb, true
                                    ),
                                array['pipeline', 'blocks', s_name, 'params', 'function', 'version']::varchar[],
                                tms_caller_tag::jsonb, true
                            )
                        WHERE id = v_id AND content -> 'pipeline' -> 'blocks' -> s_name -> 'params' ->> 'name' = 'tms-caller';

                        UPDATE versions
                        SET content = jsonb_set(
                                jsonb_set(
                                        content,
                                        array['pipeline', 'blocks', s_name, 'params', 'version']::varchar[],
                                        tms_attachment_tag::jsonb, true
                                    ),
                                array['pipeline', 'blocks', s_name, 'params', 'function', 'version']::varchar[],
                                tms_attachment_tag::jsonb, true
                            )
                        WHERE id = v_id AND content -> 'pipeline' -> 'blocks' -> s_name -> 'params' ->> 'name' = 'tms-attachment';

                        UPDATE versions
                        SET content = jsonb_set(
                                jsonb_set(
                                        content,
                                        array['pipeline', 'blocks', s_name, 'params', 'version']::varchar[],
                                        hr_departmental_awards_tag::jsonb, true
                                    ),
                                array['pipeline', 'blocks', s_name, 'params', 'function', 'version']::varchar[],
                                hr_departmental_awards_tag::jsonb, true
                            )
                        WHERE id = v_id AND content -> 'pipeline' -> 'blocks' -> s_name -> 'params' ->> 'name' = 'hr-departmental-awards';

                        UPDATE versions
                        SET content = jsonb_set(
                                jsonb_set(
                                        content,
                                        array['pipeline', 'blocks', s_name, 'params', 'version']::varchar[],
                                        kafka_producer_test_go_tag::jsonb, true
                                    ),
                                array['pipeline', 'blocks', s_name, 'params', 'function', 'version']::varchar[],
                                kafka_producer_test_go_tag::jsonb, true
                            )
                        WHERE id = v_id AND content -> 'pipeline' -> 'blocks' -> s_name -> 'params' ->> 'name' = 'kafka-producer-test-go';

                        UPDATE versions
                        SET content = jsonb_set(
                                jsonb_set(
                                        content,
                                        array['pipeline', 'blocks', s_name, 'params', 'version']::varchar[],
                                        summator_go_tag::jsonb, true
                                    ),
                                array['pipeline', 'blocks', s_name, 'params', 'function', 'version']::varchar[],
                                summator_go_tag::jsonb, true
                            )
                        WHERE id = v_id AND content -> 'pipeline' -> 'blocks' -> s_name -> 'params' ->> 'name' = 'summator-go';
                    END IF ;
                END LOOP ;
        END LOOP ;
END
$function$;

SELECT * FROM update_function_versions();

DROP FUNCTION update_function_versions;
-- +goose StatementEnd
