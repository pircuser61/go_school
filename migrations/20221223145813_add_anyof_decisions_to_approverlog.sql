-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION add_anyof_decisions_to_approver_log(id uuid) RETURNS jsonb AS
$BODY$
DECLARE
    val jsonb;

    block_list text[];
    block text; --iterator

    approver text;
    answer text;
    decision text;
    attachments jsonb;
    decision_time jsonb;
    log_entry jsonb;

    msg_detail text;
BEGIN
    -- get current json
    SELECT v.content::jsonb
    FROM variable_storage v
    WHERE v.id = $1 INTO val;

    -- approver bocks list
    SELECT array_agg(key(data))
    FROM (SELECT jsonb_each(v.content #> '{State}') AS data
          FROM variable_storage v
          WHERE v.id = $1) a
    WHERE key(data) LIKE 'approver%'
      AND value(data) ->> 'approvementRule' = 'AnyOf'
      AND value(data) ->> 'actual_approver' IS NOT NULL
      AND value(data) ->> 'decision' IS NOT NULL
    INTO block_list;

    IF block_list IS NOT NULL THEN
        FOREACH block IN ARRAY block_list
            LOOP
                approver := val #>> ('{State,' || block || ',actual_approver}')::text[];
                answer := val #>> ('{State,' || block || ',comment}')::text[];
                decision := val #>> ('{State,' || block || ',decision}')::text[];
                attachments := val #>> ('{State,' || block || ',decision_attachments}')::text[];

                SELECT to_json(updated_at) FROM variable_storage v WHERE v.id = $1 INTO decision_time;

                log_entry := ('[{
 							  "login": "' || approver || '",
 							  "comment": "' || CASE WHEN answer IS NULL THEN '' ELSE answer END || '",
 							  "decision": "' || decision || '",
 							  "log_type": "decision",
 							  "created_at": ' || CASE WHEN decision_time IS NULL THEN 'null' ELSE decision_time END || ',
 							  "attachments": ' || CASE WHEN attachments IS NULL THEN 'null' ELSE attachments END || ',
 							  "added_approvers": null
 							}]')::jsonb;

                IF val #> ('{State,' || block || ',approver_log}')::text[] IS NOT NULL THEN
                    val := jsonb_set(
                            val,
                            ('{State,' || block || ',approver_log}')::text[],
                            val #> ('{State,' || block || ',approver_log}')::text[] || log_entry
                        );
                ELSE
                    val := jsonb_set(val, ('{State,' || block || ',approver_log}')::text[], log_entry);
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

UPDATE variable_storage SET content = add_anyof_decisions_to_approver_log(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop function add_anyof_decisions_to_approver_log;
-- +goose StatementEnd
