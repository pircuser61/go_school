-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION add_decision_log_type_to_sign_log(id uuid) RETURNS jsonb AS
$BODY$
DECLARE
    val jsonb;

    block_list text[];
    block text; --iterator
    log jsonb;  --iterator
    new_log jsonb;

    msg_detail text;
BEGIN
    -- get current json
    SELECT v.content::jsonb, to_json(updated_at)
    FROM variable_storage v
    WHERE v.id = $1 INTO val;

    -- approver blocks list
    SELECT array_agg(key(data))
    FROM (SELECT jsonb_each(v.content #> '{State}') AS data
          FROM variable_storage v
          WHERE v.id = $1) a
    WHERE key (data) LIKE 'sign%'
      AND value (data) -> 'sign_log' IS NOT NULL
    INTO block_list;

    IF block_list IS NOT NULL THEN
        FOREACH block IN ARRAY block_list
            LOOP
                IF val #> ('{State,' || block || ',sign_log}')::text[] IS NOT NULL THEN
                    new_log := ('[]')::jsonb;

                    FOR log IN (SELECT jsonb_array_elements(val #> ('{State,' || block || ',sign_log}')::text[]) element)
                        LOOP
                            IF log -> 'log_type' IS NULL THEN
                                new_log := new_log || jsonb_set(log, ('{log_type}')::text[], '"decision"', true);
                            ELSE
                                new_log := new_log || log;
                            END IF;
                        END LOOP;

                    val:= jsonb_set(val, ('{State,' || block || ',sign_log}')::text[], new_log);
                END IF;
            END LOOP;
    END IF;

RETURN val;

EXCEPTION WHEN OTHERS THEN
    GET STACKED DIAGNOSTICS msg_detail = PG_EXCEPTION_DETAIL;
    RAISE NOTICE 'ID: %, %, %, %', id, SQLERRM, SQLSTATE, msg_detail;
    RETURN val;
END
$BODY$
    LANGUAGE plpgsql;

UPDATE variable_storage
SET content = add_decision_log_type_to_sign_log(id)
WHERE id IN (SELECT id
             FROM (SELECT id, JSONB_EACH(content -> 'State') as block
                   FROM variable_storage
                   WHERE step_type = 'sign') a
             WHERE key (block) LIKE 'sign%'
                AND value (block) -> 'sign_log' IS NOT NULL);

DROP FUNCTION add_decision_log_type_to_sign_log;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION add_decision_log_type_to_sign_log_rollback(id uuid) RETURNS jsonb AS
$BODY$
DECLARE
    val jsonb;

    block_list text[];
    block text; --iterator
    log jsonb;  --iterator
    new_log jsonb;

    msg_detail text;
BEGIN
    -- get current json
    SELECT v.content::jsonb, to_json(updated_at)
    FROM variable_storage v
    WHERE v.id = $1 INTO val;

    -- approver blocks list
    SELECT array_agg(key(data))
    FROM (SELECT jsonb_each(v.content #> '{State}') AS data
          FROM variable_storage v
          WHERE v.id = $1) a
    WHERE key(data) LIKE 'sign%'
      AND value(data) -> 'sign_log' IS NOT NULL
    INTO block_list;

    IF block_list IS NOT NULL THEN
        FOREACH block IN ARRAY block_list
            LOOP
                IF val #> ('{State,' || block || ',sign_log}')::text[] IS NOT NULL THEN
                    new_log := ('[]')::jsonb;

                    FOR log IN (SELECT jsonb_array_elements(val #> ('{State,' || block || ',sign_log}')::text[]) element)
                        LOOP
                            IF log -> 'log_type' = '"decision"' THEN
                                new_log := new_log || log - 'log_type';
                            ELSE
                                new_log := new_log || log;
                            END IF;
                        END LOOP;

                        val := jsonb_set(val, ('{State,' || block || ',sign_log}')::text[], new_log);
                END IF;
            END LOOP;
    END IF;

RETURN val;

EXCEPTION WHEN OTHERS THEN
    GET STACKED DIAGNOSTICS msg_detail = PG_EXCEPTION_DETAIL;
    RAISE NOTICE 'ID: %, %, %, %', id, SQLERRM, SQLSTATE, msg_detail;
    RETURN val;
END
$BODY$
    LANGUAGE plpgsql;

UPDATE variable_storage
SET content = add_decision_log_type_to_sign_log_rollback(id)
WHERE id IN (SELECT id
             FROM (SELECT id, JSONB_EACH(content -> 'State') as block
                   FROM variable_storage WHERE step_type = 'sign') a
             WHERE key(block) LIKE 'sign%'
              AND value(block) -> 'sign_log' IS NOT NULL);

DROP FUNCTION add_decision_log_type_to_sign_log_rollback;
-- +goose StatementEnd
