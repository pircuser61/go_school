package db

import (
	c "context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"golang.org/x/exp/slices"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	ActionTypePrimary   = "primary"
	ActionTypeSecondary = "secondary"
)

func uniqueActionsByRole(loginsIn, stepType string, finished, acted bool) string {
	statuses := "('running', 'idle', 'ready')"
	if finished {
		statuses = "('finished', 'cancel', 'no_success', 'error')"
	}
	memberActed := ""
	if acted {
		memberActed = "AND m.is_acted = true"
	}
	// nolint:gocritic
	// language=PostgreSQL
	return fmt.Sprintf(`WITH actions AS (
    SELECT vs.work_id                                                                                 AS work_id
         , vs.step_name                                                                               AS block_id
         , CASE WHEN vs.status IN ('running', 'idle','ready') THEN m.actions ELSE '{}' END AS action
         , CASE WHEN vs.status IN ('running', 'idle','ready') THEN m.params ELSE '{}' END  AS params
    FROM members m
             JOIN variable_storage vs on vs.id = m.block_id
             JOIN works w on vs.work_id = w.id
    JOIN lateral (SELECT vs2.step_name, max(vs2.time) mt
                                        from variable_storage vs2
                                        where vs2.work_id = w.id
                                        group by vs2.step_name) ab 
                               on ab.mt = vs.time AND ab.step_name = vs.step_name
    WHERE m.login IN %s
      AND vs.step_type = '%s'
      AND vs.status IN %s
      AND w.child_id IS NULL
		%s
      --unique-actions-filter--
)
     , unique_actions AS (
    SELECT actions.work_id AS work_id, JSONB_AGG(jsonb_actions.actions) AS actions
    FROM actions
             LEFT JOIN LATERAL (SELECT jsonb_build_object(
                                               'block_id', actions.block_id,
                                               'actions', actions.action,
                                               'params', actions.params) as actions) jsonb_actions ON TRUE
    GROUP BY actions.work_id
)`, loginsIn, stepType, statuses, memberActed)
}

func uniqueActiveActions(approverLogins, executionLogins []string, currentUser, workNumber string) string {
	var approverLoginsIn = buildInExpression(approverLogins)
	var executionLoginsIn = buildInExpression(executionLogins)

	return fmt.Sprintf(`WITH actions AS (
    SELECT vs.work_id AS work_id
         , vs.step_name AS block_id
         , m.is_initiator
         , CASE WHEN vs.status IN ('running', 'idle') AND NOT m.finished THEN m.actions ELSE '{}' END AS action
         , CASE WHEN vs.status IN ('running', 'idle') AND NOT m.finished THEN m.params ELSE '{}' END  AS params
    FROM members m
             JOIN variable_storage vs on vs.id = m.block_id
             JOIN works w on vs.work_id = w.id
    WHERE ((m.login = '%s' AND vs.step_type = 'form')
        OR (m.login = '%s' AND vs.step_type = 'sign')
        OR (m.login IN %s AND vs.step_type = 'approver')
        OR (m.login IN %s AND vs.step_type = 'execution'))
      AND w.work_number = '%s'
      AND vs.status IN ('running', 'idle', 'ready')
      AND w.child_id IS NULL
)
   , unique_actions AS (
    SELECT actions.work_id AS work_id, JSONB_AGG(jsonb_actions.actions) AS actions
    FROM actions
             LEFT JOIN LATERAL (SELECT jsonb_build_object(
                                               'block_id', actions.block_id,
                                               'actions', actions.action,
                                               'params', actions.params) as actions) jsonb_actions ON TRUE
    GROUP BY actions.work_id
)`, currentUser, currentUser, approverLoginsIn, executionLoginsIn, workNumber)
}

func buildInExpression(items []string) string {
	const (
		OpenParentheses   = "("
		ClosedParentheses = ")"
		Separator         = ","
		SingleQuote       = "'"
	)

	var sb strings.Builder

	sb.WriteString(OpenParentheses)
	for idx, item := range items {
		sb.WriteString(SingleQuote)
		sb.WriteString(item)
		sb.WriteString(SingleQuote)

		if idx < len(items)-1 {
			sb.WriteString(Separator)
		}
	}
	sb.WriteString(ClosedParentheses)

	return sb.String()
}

func getUniqueActions(selectFilter string, logins []string) string {
	var loginsIn = buildInExpression(logins)

	switch selectFilter {
	case entity.SelectAsValApprover:
		return uniqueActionsByRole(loginsIn, "approver", false, false)
	case entity.SelectAsValFinishedApprover:
		return uniqueActionsByRole(loginsIn, "approver", true, true)
	case entity.SelectAsValExecutor:
		return uniqueActionsByRole(loginsIn, "execution", false, false)
	case entity.SelectAsValFinishedExecutor:
		return uniqueActionsByRole(loginsIn, "execution", true, true)
	case entity.SelectAsValFormExecutor:
		return uniqueActionsByRole(loginsIn, "form", false, false)
	case entity.SelectAsValFinishedFormExecutor:
		return uniqueActionsByRole(loginsIn, "form", true, true)
	case entity.SelectAsValQueueExecutor:
		q := uniqueActionsByRole(loginsIn, "execution", false, false)
		q = strings.Replace(q, "--unique-actions-filter--",
			"AND vs.content -> 'State' -> vs.step_name ->> 'is_taken_in_work' = 'false' --unique-actions-filter--", 1)
		return q
	case entity.SelectAsValInWorkExecutor:
		q := uniqueActionsByRole(loginsIn, "execution", false, false)
		q = strings.Replace(q, "--unique-actions-filter--",
			"AND vs.content -> 'State' -> vs.step_name ->> 'is_taken_in_work' = 'true' --unique-actions-filter--", 1)
		return q
	case entity.SelectAsValSignerPhys:
		q := uniqueActionsByRole(loginsIn, "sign", false, false)
		q = strings.Replace(q, "--unique-actions-filter--", "AND vs.content -> 'State' -> vs.step_name ->> 'signature_type' in ('pep', 'unep') --unique-actions-filter--", 1)
		return q
	case entity.SelectAsValFinishedSignerPhys:
		q := uniqueActionsByRole(loginsIn, "sign", true, true)
		q = strings.Replace(q, "--unique-actions-filter--", "AND vs.content -> 'State' -> vs.step_name ->> 'signature_type' in ('pep', 'unep') --unique-actions-filter--", 1)
		return q
	case entity.SelectAsValSignerJur:
		q := uniqueActionsByRole(loginsIn, "sign", false, false)
		q = strings.Replace(q, "--unique-actions-filter--", "AND vs.content -> 'State' -> vs.step_name ->> 'signature_type' = 'ukep' --unique-actions-filter--", 1)
		return q
	case entity.SelectAsValFinishedSignerJur:
		q := uniqueActionsByRole(loginsIn, "sign", true, true)
		q = strings.Replace(q, "--unique-actions-filter--", "AND vs.content -> 'State' -> vs.step_name ->> 'signature_type' = 'ukep' --unique-actions-filter--", 1)
		return q
	case entity.SelectAsValInitiators:
		return fmt.Sprintf(`WITH unique_actions AS (
			SELECT id AS work_id, '[]' AS actions
			FROM works
			WHERE status = 1 AND author IN %s AND child_id IS NULL
		)`, loginsIn)
	case entity.SelectAsValGroupExecutor:
		return `WITH unique_actions AS (
			SELECT w.id AS work_id, '[]' AS actions
			FROM works w
			JOIN variable_storage vs
				ON w.id = vs.work_id
			JOIN members m
				ON vs.id = m.block_id
			WHERE w.status = 1 AND w.child_id IS NULL
				AND vs.step_type = 'execution'
				AND m.execution_group_member = true
		)`
	case entity.SelectAsValFinishedGroupExecutor:
		q := uniqueActionsByRole(loginsIn, "execution", true, false)
		return strings.Replace(q, "--unique-actions-filter--", "AND m.execution_group_member = true", 1)
	default:
		return fmt.Sprintf(`WITH unique_actions AS (
    SELECT id AS work_id, '[]' AS actions
    FROM works
    WHERE author IN %s AND child_id IS NULL
)`, loginsIn)
	}
}

//nolint:gocritic,gocyclo //filters
func compileGetTasksQuery(fl entity.TaskFilter, delegations []string) (q string, args []interface{}) {
	// nolint:gocritic
	// language=PostgreSQL
	q = `
		[with_variable_storage]
		SELECT 
			w.id,
			w.started_at,
			w.started_at,
			ws.name,
			w.human_status, 
			w.debug, 
			w.parameters, 
			w.author, 
			w.version_id,
			w.work_number,
			p.name,
			CASE 
			    WHEN w.run_context -> 'initial_application' -> 'custom_title' IS NULL
			        THEN ''
			        ELSE w.run_context -> 'initial_application' ->> 'custom_title'
			END,
			w.run_context -> 'initial_application' -> 'is_test_application',
			COALESCE(w.run_context -> 'initial_application' ->> 'description',
                COALESCE(descr.description, '')),
			COALESCE(descr.blueprint_id, ''),
			count(*) over() as total,
			w.rate,
			w.rate_comment,
		    ua.actions
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN pipelines p ON p.id = v.pipeline_id
		JOIN work_status ws ON w.status = ws.id
		JOIN unique_actions ua ON ua.work_id = w.id
		[join_variable_storage]
		LEFT JOIN LATERAL (
			SELECT work_id, 
				content::json->'State'->step_name->>'description' description,
				content::json->'State'->step_name->>'blueprint_id' blueprint_id
			FROM variable_storage vs
			WHERE vs.work_id = w.id AND vs.step_type = 'servicedesk_application' AND vs.status != 'skipped'
			LIMIT 1
		) descr ON descr.work_id = w.id
		WHERE w.child_id IS NULL`

	order := "ASC"
	if fl.Order != nil {
		order = *fl.Order
	}

	if fl.InitiatorLogins != nil && len(*fl.InitiatorLogins) > 0 {
		q = fmt.Sprintf("%s %s", getUniqueActions("initiators", *fl.InitiatorLogins), q)
	} else if fl.SelectAs != nil {
		q = fmt.Sprintf("%s %s", getUniqueActions(*fl.SelectAs, delegations), q)
	} else if fl.SelectFor != nil {
		q = fmt.Sprintf("%s %s", getUniqueActions(*fl.SelectFor, delegations), q)
	} else {
		q = fmt.Sprintf("%s %s", getUniqueActions("", delegations), q)
	}

	if fl.SignatureCarrier != nil && *fl.SelectAs == entity.SelectAsValSignerJur {
		q = strings.Replace(q, "--unique-actions-filter--",
			fmt.Sprintf("AND vs.content -> 'State' -> vs.step_name ->> 'signature_carrier' = '%s' --unique-actions-filter--",
				*fl.SignatureCarrier),
			1)
	}

	if fl.TaskIDs != nil {
		args = append(args, fl.TaskIDs)
		q = fmt.Sprintf("%s AND w.work_number = ANY($%d)", q, len(args))
	}

	if fl.Name != nil {
		name := strings.Replace(*fl.Name, "_", "!_", -1)
		name = strings.Replace(name, "%", "!%", -1)
		args = append(args, name)
		q = fmt.Sprintf(`%s AND ((p.name ILIKE '%%' || $%d || '%%' ESCAPE '!') 
							OR (w.work_number ILIKE '%%' || $%d || '%%'  ESCAPE '!') 
							OR (w.run_context -> 'initial_application' ->> 'custom_title' ILIKE '%%' || $%d || '%%'  ESCAPE '!') )`,
			q, len(args), len(args), len(args))
	}
	if fl.Created != nil {
		args = append(args, time.Unix(int64(fl.Created.Start), 0).UTC(), time.Unix(int64(fl.Created.End), 0).UTC())
		q = fmt.Sprintf("%s AND w.started_at BETWEEN $%d AND $%d", q, len(args)-1, len(args))
	}

	if fl.Archived != nil {
		switch *fl.Archived {
		case true:
			q = fmt.Sprintf("%s AND (w.archived = true OR (now()::TIMESTAMP - w.finished_at::TIMESTAMP) > '3 days')", q)
		case false:
			q = fmt.Sprintf(`%s AND (w.finished_at IS NULL 
							OR (w.archived = false AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days'))`, q)
		}
	}

	if fl.ForCarousel != nil && *fl.ForCarousel {
		q = fmt.Sprintf("%s AND ((w.human_status='done' AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days')", q)
		q = fmt.Sprintf("%s OR w.human_status = 'wait')", q)
	}

	if fl.Status != nil {
		q = fmt.Sprintf("%s AND (w.human_status IN (%s))", q, *fl.Status)
	}

	if fl.Receiver != nil {
		args = append(args, *fl.Receiver)
		q = fmt.Sprintf("%s AND w.author=$%d ", q, len(args))
	}

	if fl.Initiator != nil {
		q = fmt.Sprintf("%s AND w.author IN %s", q, buildInExpression(*fl.Initiator))
	}

	if (fl.ProcessingLogins != nil || fl.ProcessingGroupIds != nil) ||
		fl.ExecutorTypeAssigned != nil {
		q = getProcessingSteps(q, &fl)
	}

	if fl.SelectFor != nil && (fl.ProcessedLogins != nil || fl.ProcessedGroupIds != nil) {
		q = getProcessedSteps(q, &fl)
	}

	if order != "" {
		q = fmt.Sprintf("%s\n ORDER BY w.started_at %s", q, order)
	}

	if fl.Offset != nil {
		args = append(args, *fl.Offset)
		q = fmt.Sprintf("%s\n OFFSET $%d", q, len(args))
	}

	if fl.Limit != nil {
		args = append(args, *fl.Limit)
		q = fmt.Sprintf("%s\n LIMIT $%d", q, len(args))
	}

	q = strings.Replace(q, "[with_variable_storage]", "", 1)
	q = strings.Replace(q, "[join_variable_storage]", "", 1)

	return q, args
}

func getProcessingSteps(q string, fl *entity.TaskFilter) string {
	varStorage := `, var_storage as (
		SELECT DISTINCT work_id FROM variable_storage
		WHERE work_id IS NOT NULL AND status IN ('running', 'idle', 'processing')`

	varStorage = addAssignType(varStorage, fl.CurrentUser, fl.ExecutorTypeAssigned)
	varStorage = addProcessingLogins(varStorage, fl.SelectAs, fl.ProcessingLogins)
	varStorage = addProcessingGroups(varStorage, fl.SelectAs, fl.ProcessingGroupIds)

	varStorage += ")"

	q = strings.Replace(q, "[with_variable_storage]", varStorage, 1)
	q = fmt.Sprintf("%s AND w.status = 1", q)
	q = strings.Replace(q, "[join_variable_storage]", "JOIN var_storage vs ON vs.work_id = w.id ", 1)

	return q
}

func addAssignType(q, login string, typeAssign *string) string {
	if typeAssign == nil {
		return q
	}

	if *typeAssign == entity.AssignedToMe {
		q = fmt.Sprintf(`%s AND step_type = 'execution' 
			AND content -> 'State' -> step_name -> 'change_executors_logs' @> '[{"new_login": "%s"}]'`,
			q,
			login,
		)
	}

	if *typeAssign == entity.AssignedByMe {
		q = fmt.Sprintf(`%s AND step_type = 'execution' 
			AND content -> 'State' -> step_name -> 'change_executors_logs' @> '[{"old_login": "%s"}]'`,
			q,
			login,
		)
	}

	return q
}

func addProcessingLogins(q string, selectAs *string, logins *[]string) string {
	if selectAs == nil || logins == nil || len(*logins) == 0 {
		return q
	}

	ls := *logins
	ls = utils.UniqueStrings(ls)

	stepType := getStepTypeBySelectForFilter(*selectAs)

	return fmt.Sprintf(`
		%s AND step_type = '%s' AND content -> 'State' -> step_name -> '%s' ?| '%s'`,
		q,
		stepType,
		getActorsNameByStepType(stepType),
		"{"+strings.Join(ls, ",")+"}",
	)
}

func addProcessingGroups(q string, selectAs *string, groupIds *[]string) string {
	if selectAs == nil || groupIds == nil || len(*groupIds) == 0 {
		return q
	}

	ids := *groupIds
	for i := range ids {
		ids[i] = fmt.Sprintf("'%s'", ids[i])
	}

	stepType := getStepTypeBySelectForFilter(*selectAs)

	return fmt.Sprintf(`%s AND step_type = '%s' AND content -> 'State' -> step_name ->> '%s'::varchar IN(%s)`,
		q,
		stepType,
		getGroupActorsNameByStepType(stepType),
		strings.Join(ids, ","),
	)
}

func getStepTypeBySelectForFilter(selectFor string) string {
	switch selectFor {
	case "queue_executor", "in_work_executor", "finished_executor", "group_executor", "finished_group_executor":
		return "execution"
	}
	return ""
}

func getActorsNameByStepType(stepName string) string {
	switch stepName {
	case "execution":
		return "executors"
	case "approver":
		return "approvers"
	case "form":
		return "executors"
	}
	return ""
}

func getGroupActorsNameByStepType(stepName string) string {
	switch stepName {
	case "execution":
		return "executors_group_id"
	case "approver":
		return "approvers_group_id"
	}
	return ""
}

func getProcessedSteps(q string, fl *entity.TaskFilter) string {
	varStorage := `, var_storage as (
                SELECT DISTINCT work_id FROM variable_storage                                                                                                                                                                                              
                WHERE work_id IS NOT NULL AND status IN ('finished', 'cancel', 'no_success', 'error')`

	varStorage = addProcessedLogins(varStorage, fl.SelectFor, fl.ProcessedLogins)
	varStorage = addProcessedGroups(varStorage, fl.SelectFor, fl.ProcessedGroupIds)

	varStorage += ")"

	q = strings.Replace(q, "[with_variable_storage]", varStorage, 1)
	q = strings.Replace(q, "[join_variable_storage]", "JOIN var_storage vs ON vs.work_id = w.id ", 1)

	return q
}

func addProcessedLogins(q string, selectFor *string, logins *[]string) string {
	if selectFor == nil || logins == nil || len(*logins) == 0 {
		return q
	}

	ls := *logins
	ls = utils.UniqueStrings(ls)

	stepType := getStepTypeBySelectForFilter(*selectFor)

	return fmt.Sprintf(`
		%s AND step_type = '%s' AND content -> 'State' -> step_name -> '%s' ?| '%s'`,
		q,
		stepType,
		getActorsNameByStepType(stepType),
		"{"+strings.Join(ls, ",")+"}",
	)
}

func addProcessedGroups(q string, selectFor *string, groupIds *[]string) string {
	if selectFor == nil || groupIds == nil || len(*groupIds) == 0 {
		return q
	}

	ids := *groupIds
	for i := range ids {
		ids[i] = fmt.Sprintf("'%s'", ids[i])
	}

	stepType := getStepTypeBySelectForFilter(*selectFor)

	return fmt.Sprintf(`%s AND step_type = '%s' AND content -> 'State' -> step_name ->> '%s'::varchar IN(%s)`,
		q,
		stepType,
		getGroupActorsNameByStepType(stepType),
		strings.Join(ids, ","),
	)
}

func (db *PGCon) GetAdditionalForms(workNumber, nodeName string) ([]string, error) {
	const q = `
		WITH content as (
	    SELECT jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> $2 -> 'params' -> 'forms_accessibility') as rules
	    FROM versions
	    WHERE id = (SELECT version_id FROM works WHERE work_number = $1 AND child_id IS NULL)
	
	    UNION
	
	    SELECT jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> $2 -> 'params' -> 'formsAccessibility') as rules
	    FROM versions
	    WHERE id = (SELECT version_id FROM works WHERE work_number = $1 AND child_id IS NULL)
		),
	     actual_work_id as (
	         SELECT id
	         FROM works
	         WHERE work_number = $1
	           AND child_id IS NULL
	     ),
	     actual_step_name as (
	         SELECT rules ->> 'node_id' as rule
	         FROM content
	         WHERE rules ->> 'accessType' != 'None'
	     )
		SELECT content -> 'State' -> vs1.step_name ->> 'description'
		FROM variable_storage vs1
		INNER JOIN (
		    SELECT step_name, max(time) AS max_data
		    FROM variable_storage
		    WHERE work_id = (SELECT id from actual_work_id)
		    GROUP BY step_name
		) vs2
		    ON vs1.time = vs2.max_data
		        AND vs1.step_name = vs2.step_name
		WHERE vs1.work_id = (SELECT id from actual_work_id) 
			AND vs1.step_name in (SELECT rule FROM actual_step_name)
		ORDER BY time;`
	ff := make([]string, 0)
	rows, err := db.Connection.Query(c.Background(), q, workNumber, nodeName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ff, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var form string
		if scanErr := rows.Scan(&form); scanErr != nil {
			return nil, scanErr
		}
		ff = append(ff, form)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return ff, nil
}

func (db *PGCon) GetTaskFormSchemaID(workNumber, formID string) (string, error) {
	q := `SELECT content #> '{pipeline,blocks}' -> $1 #>> '{params,schema_id}'
FROM versions
WHERE id = (SELECT version_id FROM works WHERE work_number = $2 AND child_id IS NULL)`

	var id string
	if err := db.Connection.QueryRow(c.Background(), q, formID, workNumber).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (db *PGCon) GetApplicationData(workNumber string) (string, error) {
	const q = `
	SELECT coalesce(w.run_context -> 'initial_application' ->> 'description',
                coalesce(vs.content -> 'State' -> 'servicedesk_application_0' ->> 'description', ''))
FROM works w
    LEFT JOIN variable_storage vs
        ON w.id = vs.work_id AND vs.step_type = 'servicedesk_application'
WHERE w.work_number = $1
    AND w.child_id IS NULL`

	var descr string
	if err := db.Connection.QueryRow(c.Background(), q, workNumber).Scan(&descr); err != nil {
		return "", err
	}
	return descr, nil
}

//nolint:gocritic //filters
func (db *PGCon) GetTasks(ctx c.Context, filters entity.TaskFilter, delegations []string) (*entity.EriusTasksPage, error) {
	ctx, span := trace.StartSpan(ctx, "db.pg_get_tasks")
	defer span.End()

	q, args := compileGetTasksQuery(filters, delegations)

	tasks, err := db.getTasks(ctx, &filters, delegations, q, args)
	if err != nil {
		return nil, err
	}

	if len(tasks.Tasks) == 0 {
		return &entity.EriusTasksPage{Tasks: []entity.EriusTask{}}, nil
	}

	taskIDs := make([]string, 0, len(tasks.Tasks))
	for i, task := range tasks.Tasks {
		taskIDs = append(taskIDs, task.ID.String())

		steps, getTaskErr := db.GetTaskSteps(ctx, tasks.Tasks[i].ID)
		if getTaskErr != nil {
			return nil, getTaskErr
		}
		tasks.Tasks[i].Steps = steps
	}

	q = `
	WITH blocks_with_filtered_forms AS (
		SELECT work_id, value(blocks) AS blocks
		FROM (SELECT work_id, jsonb_each(state) AS blocks
			  FROM works w
					   JOIN LATERAL (
				  SELECT work_id, content::jsonb->'State' AS state
				  FROM variable_storage vs
				  WHERE vs.work_id = ANY($1)
					AND vs.work_id = w.id
				  ORDER BY vs.time DESC
				  LIMIT 1
				  ) descr ON descr.work_id = w.id
			  WHERE state IS NOT NULL AND w.id = ANY($1)) blocks_with_work_id
		WHERE key(blocks) NOT LIKE 'form%%'
		   OR (
					key(blocks) LIKE 'form%%'
				AND value(blocks) ->> 'executors' SIMILAR TO '{"(%s)": {}}'
			)
	), data AS (SELECT work_id,
					   value(jsonb_each(blocks -> 'application_body'))					  AS form_and_sd_application_body,
					   jsonb_array_elements(blocks -> 'additional_info') -> 'attachments' AS additional_info_attachments,
					   jsonb_array_elements(blocks -> 'approver_log') -> 'attachments'    AS approver_log_attachments,
					   jsonb_array_elements(blocks -> 'editing_app_log') -> 'attachments' AS editing_app_log_attachments
				FROM blocks_with_filtered_forms
 				WHERE jsonb_typeof(blocks -> 'application_body') = 'object'),
		 counts AS (SELECT
						work_id,
						SUM(CASE
                        		WHEN jsonb_typeof(form_and_sd_application_body) = 'object' 
									THEN 1
                        		WHEN jsonb_typeof(form_and_sd_application_body) = 'array'  
									THEN jsonb_array_length(form_and_sd_application_body)
                        		ELSE 0
                        	END) AS form_and_sd_count,
						SUM(coalesce(jsonb_array_length(NULLIF(additional_info_attachments, 'null')), 0)) AS additional_attachment_count,
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
					GROUP BY work_id)
	SELECT work_id,
		   form_and_sd_count + additional_attachment_count + additional_approvers_count +
		   rework_count
	FROM counts;
	`

	logins := strings.Join(delegations, "|")
	q = fmt.Sprintf(q, logins)

	rows, err := db.Connection.Query(ctx, q, taskIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		taskID             uuid.UUID
		attachmentsCount   int
		attachmentsToTasks = map[uuid.UUID]int{}
	)

	for rows.Next() {
		err = rows.Scan(&taskID, &attachmentsCount)
		if err != nil {
			return nil, err
		}

		attachmentsToTasks[taskID] = attachmentsCount
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	for i := range tasks.Tasks {
		count := attachmentsToTasks[tasks.Tasks[i].ID]
		tasks.Tasks[i].AttachmentsCount = &count
	}

	return &entity.EriusTasksPage{
		Tasks: tasks.Tasks,
		Total: tasks.Tasks[0].Total,
	}, nil
}

func (db *PGCon) GetDeadline(ctx c.Context, workNumber string) (time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_last_debug_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
    WITH blocks AS (
    	SELECT  content->'State'->step_name AS block FROM variable_storage vs WHERE work_id = (SELECT id from works WHERE work_number = $1 and child_id is null) and step_type = 'execution' and status = 'running'
	)
	SELECT coalesce(min(block ->> 'deadline'),'') FROM blocks;
  `

	row := db.Connection.QueryRow(ctx, q, workNumber)

	var deadline string
	err := row.Scan(&deadline)
	if err != nil {
		return time.Time{}, err
	}

	if deadline != "" {
		loc, _ := time.LoadLocation("Europe/Moscow")
		deadlines, deadErr := time.ParseInLocation(time.RFC3339, deadline, loc)
		if deadErr != nil {
			return time.Time{}, deadErr
		}

		return deadlines, nil
	}

	return time.Time{}, nil
}

func (db *PGCon) GetTasksCount(
	ctx c.Context,
	currentUser string,
	delegationsByApprovement,
	delegationsByExecution []string) (*entity.CountTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks_count")
	defer span.End()
	// nolint:gocritic
	// language=PostgreSQL
	q := `
WITH active_counts as (
    SELECT count(*) as active_count
    FROM works w
    WHERE author = $1
      AND (w.finished_at IS NULL OR (w.archived = false
        AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days'))
      AND child_id IS NULL    
)
   , approve_counts AS (
    SELECT count(*) OVER () as c
    FROM members m
             JOIN variable_storage vs ON vs.id = m.block_id
             JOIN works w ON vs.work_id = w.id AND w.child_id IS NULL
    WHERE vs.status IN ('running', 'idle', 'ready')
      AND m.login = ANY ($2)
      AND vs.step_type = 'approver'
    GROUP BY vs.work_id
    limit 1
)
   , execution_counts as (
    SELECT count(*) over () as c
    FROM members m
             JOIN variable_storage vs ON vs.id = m.block_id
             JOIN works w ON vs.work_id = w.id AND w.child_id IS NULL
    WHERE vs.status IN ('running', 'idle', 'ready')
      AND m.login = ANY ($3)
      AND vs.step_type = 'execution'
    GROUP BY vs.work_id
    LIMIT 1
)
   , form_counts AS (
    SELECT count(*) OVER () as c
    FROM members m
             JOIN variable_storage vs ON vs.id = m.block_id
             JOIN works w ON vs.work_id = w.id AND w.child_id IS NULL
    WHERE vs.status IN ('running', 'idle', 'ready')
      AND m.login = $1
      AND vs.step_type = 'form'
    GROUP BY vs.work_id
    LIMIT 1
)
   , sign_counts AS (
    SELECT count(*) OVER () as c
    FROM members m
             JOIN variable_storage vs ON vs.id = m.block_id
             JOIN works w ON vs.work_id = w.id AND w.child_id IS NULL
    WHERE vs.status IN ('running', 'idle', 'ready')
      AND m.login = $1
      AND vs.step_type = 'sign'
    GROUP BY vs.work_id
    LIMIT 1
)
    (
        SELECT active_count
             , coalesce(approve_counts.c, 0)
             , coalesce(execution_counts.c, 0)
             , coalesce(form_counts.c, 0)
             , coalesce(sign_counts.c, 0)
        FROM active_counts
                 LEFT JOIN approve_counts ON TRUE
                 LEFT JOIN execution_counts ON TRUE
                 LEFT JOIN form_counts ON TRUE
                 LEFT JOIN sign_counts ON TRUE
        limit 1
    );
`

	counter, err := db.getTasksCount(
		ctx, q,
		currentUser,
		delegationsByApprovement,
		delegationsByExecution)
	if err != nil {
		return nil, err
	}

	return &entity.CountTasks{
		TotalActive:       counter.totalActive,
		TotalExecutor:     counter.totalExecutor,
		TotalApprover:     counter.totalApprover,
		TotalFormExecutor: counter.totalFormExecutor,
		TotalSign:         counter.totalSign,
	}, nil
}

func (db *PGCon) GetPipelineTasks(ctx c.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_pipeline_tasks")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
			w.id, 
			w.started_at, 
			ws.name, 
			w.human_status, 
			w.debug, 
			w.parameters, 
			w.author, 
			w.version_id,
       		w.work_number
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN pipelines p ON p.id = v.pipeline_id
		JOIN work_status ws ON w.status = ws.id
		WHERE p.id = $1 AND w.child_id IS NULL
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(ctx, &entity.TaskFilter{}, []string{}, q, []interface{}{pipelineID})
}

func (db *PGCon) GetVersionTasks(ctx c.Context, versionID uuid.UUID) (*entity.EriusTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_version_tasks")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
			w.id, 
			w.started_at, 
			ws.name,
       		w.human_status,
			w.debug, 
			w.parameters,
			w.author, 
			w.version_id,
       		w.work_number
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN work_status ws ON w.status = ws.id
		WHERE v.id = $1 AND w.child_id IS NULL
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(ctx, &entity.TaskFilter{}, []string{}, q, []interface{}{versionID})
}

func (db *PGCon) GetLastDebugTask(ctx c.Context, id uuid.UUID, author string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_last_debug_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `SELECT 
			w.id, 
			w.started_at, 
			ws.name, 
       		w.human_status,
			w.debug, 
			w.parameters, 
			w.author, 
			w.version_id
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN work_status ws ON w.status = ws.id
		WHERE v.id = $1
		AND w.author = $2
		AND w.debug = true
		AND w.child_id IS NULL
		ORDER BY w.started_at DESC
		LIMIT 1`

	et := entity.EriusTask{}

	row := db.Connection.QueryRow(ctx, q, id, author)
	parameters := ""

	err := row.Scan(&et.ID, &et.StartedAt, &et.Status, &et.HumanStatus, &et.IsDebugMode, &parameters, &et.Author, &et.VersionID)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(parameters), &et.Parameters)
	if err != nil {
		return nil, err
	}

	return &et, nil
}

func (db *PGCon) GetTask(
	ctx c.Context,
	delegationsApprover,
	delegationsExecution []string,
	currentUser, workNumber string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := uniqueActiveActions(delegationsApprover, delegationsExecution, currentUser, workNumber)

	q += ` SELECT 
			w.id, 
			w.started_at, 
			w.started_at, 
			w.finished_at,
			ws.name,
			w.human_status,
			w.debug, 
			COALESCE(w.parameters, '{}') AS parameters,
			w.author,
			w.version_id,
			w.work_number,
			p.name,
			CASE 
			    WHEN run_context -> 'initial_application' -> 'custom_title' IS NULL
			        THEN ''
			        ELSE run_context -> 'initial_application' ->> 'custom_title'
			END,
			COALESCE(w.run_context -> 'initial_application' ->> 'description',
                COALESCE(descr.description, '')),
			COALESCE(descr.blueprint_id, ''),
			w.rate,
			w.rate_comment,
         	ua.actions,
 			run_context -> 'initial_application' -> 'is_test_application' as isTest,
 			w.status_comment,
			w.status_author,
 			v.content,
 			v.node_groups,
 			w.human_status_comment
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN pipelines p ON p.id = v.pipeline_id
		JOIN work_status ws ON w.status = ws.id
		LEFT JOIN unique_actions ua ON ua.work_id = w.id
		LEFT JOIN LATERAL (
			SELECT work_id, 
				content::json->'State'->step_name->>'description' description,
				content::json->'State'->step_name->>'blueprint_id' blueprint_id
			FROM variable_storage vs
			WHERE vs.work_id = w.id AND vs.step_type = 'servicedesk_application' AND vs.status != 'skipped'
			ORDER BY vs.time DESC
			LIMIT 1
		) descr ON descr.work_id = w.id
		WHERE w.work_number = $1 
			AND w.child_id IS NULL
`
	return db.getTask(ctx, []string{currentUser}, q, workNumber)
}

func (db *PGCon) getTask(ctx c.Context, delegators []string, q, workNumber string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_private")
	defer span.End()

	actionsMap, getActionsErr := db.getActionsMap(ctx)
	if getActionsErr != nil {
		return &entity.EriusTask{}, getActionsErr
	}

	et := entity.EriusTask{}

	var nullStringParameters sql.NullString
	var actionData []byte
	var nodeGroups string

	row := db.Connection.QueryRow(ctx, q, workNumber)

	err := row.Scan(
		&et.ID,
		&et.StartedAt,
		&et.LastChangedAt,
		&et.FinishedAt,
		&et.Status,
		&et.HumanStatus,
		&et.IsDebugMode,
		&nullStringParameters,
		&et.Author,
		&et.VersionID,
		&et.WorkNumber,
		&et.Name,
		&et.CustomTitle,
		&et.Description,
		&et.BlueprintID,
		&et.Rate,
		&et.RateComment,
		&actionData,
		&et.IsTest,
		&et.StatusComment,
		&et.StatusAuthor,
		&et.VersionContent,
		&nodeGroups,
		&et.HumanStatusComment,
	)
	if err != nil {
		return nil, err
	}

	et.Name = utils.MakeTaskTitle(et.Name, et.CustomTitle, et.IsTest)

	var actions []DbTaskAction
	if actionData != nil {
		if unmErr := json.Unmarshal(actionData, &actions); unmErr != nil {
			return nil, unmErr
		}
	}

	computedActions, actionsErr := db.computeActions(ctx, delegators, actions, actionsMap, et.Author)
	if actionsErr != nil {
		return nil, actionsErr
	}

	et.Actions = computedActions

	if nullStringParameters.Valid && nullStringParameters.String != "" {
		err = json.Unmarshal([]byte(nullStringParameters.String), &et.Parameters)
		if err != nil {
			return nil, err
		}
	}
	et.NodeGroup = make([]*entity.NodeGroup, 0)
	err = json.Unmarshal([]byte(nodeGroups), &et.NodeGroup)
	if err != nil {
		return nil, err
	}
	return &et, nil
}

type IgnoreActionRule struct {
	IgnoreActionId   string
	ExistingActionId string
}

func getActionsToIgnoreIfOtherExist() []IgnoreActionRule {
	return []IgnoreActionRule{
		{
			IgnoreActionId:   "additional_approvement",
			ExistingActionId: "approve",
		},
		{
			IgnoreActionId:   "additional_approvement",
			ExistingActionId: "informed",
		},
		{
			IgnoreActionId:   "additional_approvement",
			ExistingActionId: "confirm",
		},
		{
			IgnoreActionId:   "additional_approvement",
			ExistingActionId: "sign",
		},
		{
			IgnoreActionId:   "additional_approvement",
			ExistingActionId: "viewed",
		},
		{
			IgnoreActionId:   "additional_reject",
			ExistingActionId: "approve",
		},
		{
			IgnoreActionId:   "additional_reject",
			ExistingActionId: "informed",
		},
		{
			IgnoreActionId:   "additional_reject",
			ExistingActionId: "confirm",
		},
		{
			IgnoreActionId:   "additional_reject",
			ExistingActionId: "sign",
		},
		{
			IgnoreActionId:   "additional_reject",
			ExistingActionId: "viewed",
		},
		{
			IgnoreActionId:   "additional_reject",
			ExistingActionId: "reject",
		},
		{
			IgnoreActionId:   "additional_approvement",
			ExistingActionId: "reject",
		},
	}
}

func getMaxPriority(existingPriorities []entity.TaskAction) string {
	nodeTypes := map[string]int{
		"execution":   3,
		"approvement": 2,
		"sign":        1,
		"form":        0,
	}

	result := ""
	for _, v := range existingPriorities {
		if v.ButtonType != ActionTypePrimary && v.ButtonType != ActionTypeSecondary {
			continue
		}

		if nums, ok := nodeTypes[v.NodeType]; ok && nums > nodeTypes[result] {
			result = v.NodeType
		}
	}

	return result
}

func (db *PGCon) computeActions(ctx c.Context, currentUserDelegators []string, actions []DbTaskAction,
	allActions map[string]entity.TaskAction, author string) (result []entity.TaskAction, err error) {
	const (
		CancelAppId       = "cancel_app"
		CancelAppPriority = "other"
		CancelAppTitle    = "Отозвать"
		CancelAppNodeType = "common"

		RepeatAppId       = "repeat_app"
		RepeatAppPriority = "other"
		RepeatAppTitle    = "Повторить"
		RepeatAppNodeType = "common"
	)

	var computedActions = make([]entity.TaskAction, 0)
	var computedActionIds = make([]string, 0)
	var actionsToIgnore = getActionsToIgnoreIfOtherExist()

	result = make([]entity.TaskAction, 0)

	canBeRepeated := []string{
		string(entity.TaskUpdateActionReplyApproverInfo),
		string(entity.TaskUpdateActionRequestFillForm),
	}

	metActions := make(map[string]struct{})

	for _, blockActions := range actions {
		for _, action := range blockActions.Actions {
			var compositeActionId = strings.Split(action, ":")
			if len(compositeActionId) > 1 {
				id := compositeActionId[0]

				if _, ok := metActions[id]; ok && !utils.IsContainsInSlice(id, canBeRepeated) {
					continue
				}

				metActions[id] = struct{}{}

				priority := compositeActionId[1]
				actionWithPreferences := allActions[id]
				actionParams, _ := blockActions.Params[id]

				var computedAction = entity.TaskAction{
					Id:                 id,
					ButtonType:         priority,
					NodeType:           actionWithPreferences.NodeType,
					Title:              actionWithPreferences.Title,
					CommentEnabled:     actionWithPreferences.CommentEnabled,
					AttachmentsEnabled: actionWithPreferences.AttachmentsEnabled,
					IsPublic:           actionWithPreferences.IsPublic,
					Params:             actionParams,
				}

				computedActions = append(computedActions, computedAction)
				computedActionIds = append(computedActionIds, computedAction.Id)
			}
		}
	}

	maxPriority := getMaxPriority(computedActions)

	for _, a := range computedActions {
		var ignoreAction = false

		if maxPriority != "" && a.NodeType != maxPriority && (a.ButtonType == ActionTypePrimary || a.ButtonType == ActionTypeSecondary) {
			a.ButtonType = "other"
		}

		for _, actionRule := range actionsToIgnore {
			if a.Id == actionRule.IgnoreActionId && slices.Contains(computedActionIds, actionRule.ExistingActionId) {
				ignoreAction = true
				break
			}
		}

		if !ignoreAction {
			result = append(result, a)
		}
	}

	ui, err := user.GetEffectiveUserInfoFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	isInitiator := ui.Username == author

	if isInitiator {
		var cancelAppAction = entity.TaskAction{
			Id:                 CancelAppId,
			ButtonType:         CancelAppPriority,
			NodeType:           CancelAppNodeType,
			Title:              CancelAppTitle,
			CommentEnabled:     true,
			AttachmentsEnabled: false,
		}

		var repeatAppAction = entity.TaskAction{
			Id:                 RepeatAppId,
			ButtonType:         RepeatAppPriority,
			NodeType:           RepeatAppNodeType,
			Title:              RepeatAppTitle,
			CommentEnabled:     true,
			AttachmentsEnabled: false,
		}

		result = append(result, cancelAppAction, repeatAppAction)
	}

	return result, nil
}

type tasksCounter struct {
	totalActive       int
	totalExecutor     int
	totalApprover     int
	totalFormExecutor int
	totalSign         int
}

func (db *PGCon) getTasksCount(
	ctx c.Context,
	q, currentUser string,
	usernamesByApprovement, usernamesByExecution []string) (*tasksCounter, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks_count")
	defer span.End()

	counter := &tasksCounter{}

	if scanErr := db.Connection.QueryRow(ctx, q, currentUser, usernamesByApprovement, usernamesByExecution).
		Scan(
			&counter.totalActive,
			&counter.totalApprover,
			&counter.totalExecutor,
			&counter.totalFormExecutor,
			&counter.totalSign,
		); scanErr != nil {
		return counter, scanErr
	}

	return counter, nil
}

//nolint:gocyclo //its ok here
func (db *PGCon) getTasks(ctx c.Context, filters *entity.TaskFilter,
	delegatorsWithUser []string, q string, args []interface{}) (*entity.EriusTasks, error) {
	ctx, span := trace.StartSpan(ctx, "db.pg_get_tasks")
	defer span.End()

	ets := entity.EriusTasks{
		Tasks: make([]entity.EriusTask, 0),
	}

	rows, err := db.Connection.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actionsMap, getActionsErr := db.getActionsMap(ctx)
	if getActionsErr != nil {
		return &entity.EriusTasks{}, getActionsErr
	}

	for rows.Next() {
		et := entity.EriusTask{}
		var nullStringParameters sql.NullString
		var actionData []byte

		err = rows.Scan(
			&et.ID,
			&et.StartedAt,
			&et.LastChangedAt,
			&et.Status,
			&et.HumanStatus,
			&et.IsDebugMode,
			&nullStringParameters,
			&et.Author,
			&et.VersionID,
			&et.WorkNumber,
			&et.Name,
			&et.CustomTitle,
			&et.IsTest,
			&et.Description,
			&et.BlueprintID,
			&et.Total,
			&et.Rate,
			&et.RateComment,
			&actionData,
		)

		if err != nil {
			return nil, err
		}

		et.Name = utils.MakeTaskTitle(et.Name, et.CustomTitle, et.IsTest)

		if nullStringParameters.Valid && nullStringParameters.String != "" {
			err = json.Unmarshal([]byte(nullStringParameters.String), &et.Parameters)
			if err != nil {
				return nil, err
			}
		}

		var actions []DbTaskAction
		if actionData != nil {
			if unmErr := json.Unmarshal(actionData, &actions); unmErr != nil {
				return nil, unmErr
			}
		}

		computedActions, actionsErr := db.computeActions(ctx, delegatorsWithUser, actions, actionsMap, et.Author)
		if actionsErr != nil {
			return nil, err
		}

		et.Actions = computedActions
		et.IsDelegate = filters.CurrentUser != et.Author
		ets.Tasks = append(ets.Tasks, et)
	}

	return &ets, nil
}

func (db *PGCon) GetTaskSteps(ctx c.Context, id uuid.UUID) (entity.TaskSteps, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_steps")
	defer span.End()

	el := entity.TaskSteps{}

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		SELECT 
			vs.id,
			vs.step_type,
			vs.step_name,
			vs.time, 
			vs.content, 
			COALESCE(vs.break_points, '{}') AS break_points, 
			vs.has_error,
			vs.status,
			vs.updated_at
		FROM variable_storage vs 
			WHERE work_id = $1 AND vs.status != 'skipped' AND 
			(SELECT max(time)
				 FROM variable_storage vrbs
				 WHERE vrbs.step_name = vs.step_name AND
					   vrbs.work_id = $1
				) = vs.time
		ORDER BY vs.time DESC`

	rows, err := db.Connection.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//nolint:dupl //scan
	for rows.Next() {
		s := entity.Step{}
		var content string

		err = rows.Scan(
			&s.ID,
			&s.Type,
			&s.Name,
			&s.Time,
			&content,
			&s.BreakPoints,
			&s.HasError,
			&s.Status,
			&s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		storage := store.NewStore()

		err = json.Unmarshal([]byte(content), storage)
		if err != nil {
			return nil, err
		}

		s.State = storage.State
		s.Steps = storage.Steps
		s.Errors = storage.Errors
		s.Storage = storage.Values
		el = append(el, &s)
	}

	return el, nil
}

func (db *PGCon) GetFilteredStates(ctx c.Context, steps []string, wNumber string) (
	map[string]map[string]interface{}, map[string]map[string]*time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_filtered_states")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	query := `
		SELECT step_name, vs.content-> 'State', time, updated_at
		FROM variable_storage vs 
			WHERE vs.work_id = (SELECT id FROM works 
			                 	WHERE work_number = $1 AND child_id IS NULL LIMIT 1) AND 
			vs.step_name IN %s AND 
			vs.time = (SELECT max(time) FROM variable_storage WHERE work_id = vs.work_id AND step_name = vs.step_name)
		ORDER BY vs.time DESC`

	if len(steps) == 0 {
		query = fmt.Sprintf(query, "('')")
	} else {
		query = fmt.Sprintf(query, buildInExpression(steps))
	}

	res := make([]map[string]map[string]interface{}, 0)
	rows, err := db.Connection.Query(ctx, query, wNumber)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	dates := make(map[string]map[string]*time.Time)

	for rows.Next() {
		stepName := ""
		states := make(map[string]map[string]interface{})
		var createdAt *time.Time
		var updatedAt *time.Time
		if scanErr := rows.Scan(&stepName, &states, &createdAt, &updatedAt); scanErr != nil {
			return nil, nil, scanErr
		}

		dates[stepName] = map[string]*time.Time{
			"createdAt": createdAt,
			"updatedAt": updatedAt,
		}
		res = append(res, states)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, err
	}

	return mergeStates(res, steps), dates, nil
}

func mergeStates(in []map[string]map[string]interface{}, steps []string) (res map[string]map[string]interface{}) {
	res = make(map[string]map[string]interface{})
	for i := range in {
		for stepName := range in[i] {
			if !utils.IsContainsInSlice(stepName, steps) {
				continue
			}
			if _, exists := res[stepName]; !exists {
				res[stepName] = in[i][stepName]
			}
		}
	}

	return res
}

func (db *PGCon) GetTaskHumanStatus(ctx c.Context, taskID uuid.UUID) (string, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_status")
	defer span.End()

	q := `
		SELECT human_status
		FROM works
		WHERE id = $1`

	var status string

	if err := db.Connection.QueryRow(ctx, q, taskID).Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}

func (db *PGCon) GetTaskStatus(ctx c.Context, taskID uuid.UUID) (int, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_status")
	defer span.End()

	q := `
		SELECT status
		FROM works
		WHERE id = $1`

	var status int

	if err := db.Connection.QueryRow(ctx, q, taskID).Scan(&status); err != nil {
		return -1, err
	}
	return status, nil
}

func (db *PGCon) GetTaskStatusWithReadableString(ctx c.Context, taskID uuid.UUID) (int, string, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_status")
	defer span.End()

	q := `
		SELECT w.status,
		       ws.name
		FROM works w join work_status ws on w.status =ws.id
		WHERE w.id = $1`

	var intStatus int
	var stringStatus string
	if err := db.Connection.QueryRow(ctx, q, taskID).Scan(&intStatus, &stringStatus); err != nil {
		return -1, "", err
	}
	return intStatus, stringStatus, nil
}

func (db *PGCon) getActionsMap(ctx c.Context) (actions map[string]entity.TaskAction, err error) {
	const q = `
		SELECT 
			id,
			title,
			is_public,
			comment_enabled,
			attachments_enabled,
			node_type
		FROM dict_actions`

	result := make(map[string]entity.TaskAction, 0)
	rows, err := db.Connection.Query(ctx, q)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		ta := entity.TaskAction{}

		if err := rows.Scan(
			&ta.Id,
			&ta.Title,
			&ta.IsPublic,
			&ta.CommentEnabled,
			&ta.AttachmentsEnabled,
			&ta.NodeType,
		); err != nil {
			return nil, err
		}

		result[ta.Id] = ta
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return result, nil
}

func (db *PGCon) actionsToStrings(nullStringActions []sql.NullString) []string {
	actions := make([]string, 0, len(nullStringActions))
	for _, action := range nullStringActions {
		if action.Valid {
			actions = append(actions, action.String)
		}
	}

	return actions
}

func (db *PGCon) GetMeanTaskSolveTime(ctx c.Context, pipelineId string) (
	result []entity.TaskCompletionInterval, err error) {
	const q = `
	WITH cte AS (
	SELECT
		started_at,
		finished_at,
		count(*) OVER() cnt
	FROM works w
	  JOIN versions v ON v.id = w.version_id
	  JOIN pipelines p ON p.id = v.pipeline_id
	  JOIN work_status ws ON w.status = ws.id
	WHERE p.id = $1
		AND v.is_actual = TRUE
		AND coalesce(w.run_context -> 'initial_application' -> 'is_test_application' = 'false', true)
		AND ws.name = 'finished')

	SELECT started_at, finished_at FROM cte
		WHERE cnt >= 30 ORDER BY started_at
	`

	result = make([]entity.TaskCompletionInterval, 0)

	rows, err := db.Connection.Query(ctx, q, pipelineId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		interval := entity.TaskCompletionInterval{}

		if scanErr := rows.Scan(
			&interval.StartedAt,
			&interval.FinishedAt,
		); scanErr != nil {
			return nil, scanErr
		}

		result = append(result, interval)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return result, nil
}

func (db *PGCon) CheckIsArchived(ctx c.Context, taskID uuid.UUID) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "check_is_archived")
	defer span.End()

	q := `
		SELECT archived
		FROM works
		WHERE id = $1`

	var isArchived bool
	if err := db.Connection.QueryRow(ctx, q, taskID).Scan(&isArchived); err != nil {
		return false, err
	}

	return isArchived, nil
}

func (db *PGCon) GetBlocksOutputs(ctx c.Context, blockId string) (entity.BlockOutputs, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_block_content")
	defer span.End()

	q := `
		SELECT step_name, content -> 'Values'
		FROM variable_storage
		WHERE id = $1;
	`

	blockData := struct {
		StepName        string
		VariableStorage map[string]interface{}
	}{}

	if err := db.Connection.QueryRow(ctx, q, blockId).Scan(&blockData.StepName, &blockData.VariableStorage); err != nil {
		return nil, err
	}

	blockOutputs := make(entity.BlockOutputs, 0)
	for k, v := range blockData.VariableStorage {
		blockOutputs = append(blockOutputs, entity.BlockOutputValue{
			StepName: blockData.StepName,
			Name:     k,
			Value:    v,
		})
	}

	return blockOutputs, nil
}

func (db *PGCon) GetMergedVariableStorage(ctx c.Context, workId uuid.UUID, blockIds []string) (*store.VariableStore, error) {
	ctx, span := trace.StartSpan(ctx, "get_merged_variable_storage")
	defer span.End()

	const q = `
		SELECT jsonb_merge_agg(vs.content) AS content 
			FROM variable_storage vs
    	WHERE work_id = '%s' AND step_name IN %s AND
    	  vs.time = (SELECT max(time) FROM variable_storage WHERE work_id = vs.work_id AND step_name = vs.step_name)`

	query := fmt.Sprintf(q, workId, buildInExpression(blockIds))

	var content []byte
	if err := db.Connection.QueryRow(ctx, query).Scan(&content); err != nil {
		return nil, err
	}

	storage := store.NewStore()
	if err := json.Unmarshal(content, &storage); err != nil {
		return nil, err
	}

	return storage, nil
}

func (db *PGCon) GetTasksForMonitoring(ctx c.Context, filters *entity.TasksForMonitoringFilters) (*entity.TasksForMonitoring, error) {
	ctx, span := trace.StartSpan(ctx, "get_tasks_for_monitoring")
	defer span.End()

	q := getTasksForMonitoringQuery(filters)

	rows, err := db.Connection.Query(ctx, *q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasksForMonitoring := &entity.TasksForMonitoring{
		Tasks: make([]entity.TaskForMonitoring, 0),
	}

	for rows.Next() {
		task := entity.TaskForMonitoring{}

		err = rows.Scan(&task.Status,
			&task.ProcessName,
			&task.Initiator,
			&task.WorkNumber,
			&task.StartedAt,
			&task.FinishedAt,
			&task.ProcessDeletedAt,
			&tasksForMonitoring.Total)
		if err != nil {
			return nil, err
		}

		tasksForMonitoring.Tasks = append(tasksForMonitoring.Tasks, task)
	}

	return tasksForMonitoring, nil
}

func getWorksStatusQuery(statusFilter []string) *string {
	statusQuery := `(CASE 
						WHEN w.status IN (1, 3, 5) THEN 'В работе' 
						WHEN w.status = 2 THEN 'Завершен' 
						WHEN w.status = 4 THEN 'Остановлен' 
						WHEN w.status = 6 THEN 'Отменен'
						WHEN w.status IS NULL THEN 'Неизвестный статус' END) 
						IN %s`

	statusQueryFilter := make([]string, 0, len(statusFilter))

	for i := range statusFilter {
		statusQueryFilter = append(statusQueryFilter, "'"+statusFilter[i]+"'")
	}
	v := "(" + strings.Join(statusQueryFilter, ",") + ")"

	statusQuery = fmt.Sprintf(statusQuery, v)

	return &statusQuery
}

func getTasksForMonitoringQuery(filters *entity.TasksForMonitoringFilters) *string {
	q := `
			SELECT CASE
					WHEN w.status IN (1, 3, 5) THEN 'В работе'
        			WHEN w.status = 2 THEN 'Завершен'
				    WHEN w.status = 4 THEN 'Остановлен'
			    	WHEN w.status = 6 THEN 'Отменен'
        			WHEN w.status IS NULL THEN 'Неизвестный статус'
    			END AS status,
				p.name AS process_name,
				w.author AS initiator,
				w.work_number AS work_number,
				w.started_at AS started_at,
				w.finished_at as finished_at,
				p.deleted_at as process_deleted_at,
				COUNT(*) OVER() as total
			FROM works w
			LEFT JOIN versions v on w.version_id = v.id
			LEFT JOIN pipelines p on v.pipeline_id = p.id
			WHERE w.started_at IS NOT NULL AND p.name IS NOT NULL AND v.is_hidden = false
	`

	if filters.FromDate != nil || filters.ToDate != nil {
		q = fmt.Sprintf("%s AND %s", q, getFiltersDateConditions(filters.FromDate, filters.ToDate))
	}

	if searchConditions := getFiltersSearchConditions(filters.Filter); searchConditions != "" {
		q = fmt.Sprintf("%s AND %s", q, searchConditions)
	}

	if len(filters.StatusFilter) != 0 {
		statusQuery := getWorksStatusQuery(filters.StatusFilter)
		q = fmt.Sprintf("%s AND %s", q, *statusQuery)
	}

	if filters.SortColumn != nil && filters.SortOrder != nil {
		q = fmt.Sprintf("%s ORDER BY %s %s", q, *filters.SortColumn, *filters.SortOrder)
	} else {
		q = fmt.Sprintf("%s ORDER BY %s %s", q, "w.started_at", "DESC")
	}

	if filters.Page != nil && filters.PerPage != nil {
		q = fmt.Sprintf("%s OFFSET %d", q, *filters.Page**filters.PerPage)
	}

	if filters.PerPage != nil {
		q = fmt.Sprintf("%s LIMIT %d", q, *filters.PerPage)
	}

	return &q
}

func getFiltersSearchConditions(filter *string) string {
	if filter == nil {
		return ""
	}
	escapeFilter := strings.Replace(*filter, "_", "!_", -1)
	escapeFilter = strings.Replace(escapeFilter, "%", "!%", -1)
	return fmt.Sprintf(`
		(w.work_number ILIKE '%%%s%%' ESCAPE '!' OR
		 p.name ILIKE '%%%s%%' ESCAPE '!' OR
		 w.author ILIKE '%%%s%%' ESCAPE '!')`,
		escapeFilter, escapeFilter, escapeFilter)
}

func getFiltersDateConditions(dateFrom, dateTo *string) string {
	conditions := make([]string, 0)

	if dateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("w.started_at >= '%s'::timestamptz", *dateFrom))
	}

	if dateTo != nil {
		conditions = append(conditions, fmt.Sprintf("w.started_at <= '%s'::timestamptz", *dateTo))
	}

	return strings.Join(conditions, " AND ")
}

func (db *PGCon) GetBlockInputs(ctx c.Context, blockName, workNumber string) (entity.BlockInputs, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_block_inputs")
	defer span.End()

	blockInputs := make(entity.BlockInputs, 0)
	params := make(map[string]interface{}, 0)

	version, err := db.GetVersionByWorkNumber(ctx, workNumber)
	if err != nil {
		return blockInputs, nil
	}

	const q = `
		SELECT content -> 'pipeline' -> 'blocks' -> $1 -> 'params'
		FROM versions
		WHERE id = $2;
	`

	if err = db.Connection.QueryRow(ctx, q, blockName, version.VersionID).Scan(&params); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return blockInputs, nil
		}
		return nil, err
	}

	for i := range params {
		blockInputs = append(blockInputs, entity.BlockInputValue{
			Name:  i,
			Value: params[i],
		})
	}

	return blockInputs, nil
}

func (db *PGCon) GetBlockOutputs(ctx c.Context, blockId, blockName string) (entity.BlockOutputs, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_block_outputs")
	defer span.End()

	blockOutputs := make(entity.BlockOutputs, 0)
	blocksOutputs, err := db.GetBlocksOutputs(ctx, blockId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return blockOutputs, nil
		}
		return nil, err
	}

	for i := range blocksOutputs {
		if strings.Contains(blocksOutputs[i].Name, blockName) {
			blockOutputs = append(blockOutputs, entity.BlockOutputValue{
				Name:  strings.Replace(blocksOutputs[i].Name, blockName+".", "", 1),
				Value: blocksOutputs[i].Value,
			})
		}
	}

	return blockOutputs, nil
}

func (db *PGCon) GetBlockState(ctx c.Context, blockId string) (entity.BlockState, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_block_state")
	defer span.End()

	state := make(entity.BlockState, 0)
	params := make(map[string]interface{}, 0)

	const q = `
		SELECT content -> 'State' -> step_name
		FROM variable_storage
		WHERE id = $1;
	`

	if err := db.Connection.QueryRow(ctx, q, blockId).Scan(&params); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return state, nil
		}
		return nil, err
	}

	for i := range params {
		state = append(state, entity.BlockStateValue{
			Name:  i,
			Value: params[i],
		})
	}

	return state, nil
}

func (db *PGCon) CheckBlockForHiddenFlag(ctx c.Context, blockId string) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "check_task_node_for_hidden_flag_monitoring")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT v.is_hidden
		from variable_storage vs
		    join works w on w.id = vs.work_id
    		join versions v on w.version_id = v.id
		where vs.id = $1`

	var res bool
	if err := db.Connection.QueryRow(ctx, q, blockId).Scan(&res); err != nil {
		return false, err
	}

	return res, nil
}

func (db *PGCon) CheckTaskForHiddenFlag(ctx c.Context, workNumber string) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "check_task_for_hidden_flag_monitoring")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT v.is_hidden
		from works w
    		join versions v on w.version_id = v.id
		where w.work_number = $1 AND w.child_id is null`

	var res bool
	if err := db.Connection.QueryRow(ctx, q, workNumber).Scan(&res); err != nil {
		return false, err
	}

	return res, nil
}

func (db *PGCon) GetTaskMembers(ctx c.Context, workNumber string, fromActiveNodes bool) ([]DbMember, error) {
	q := `SELECT m.login, vs.step_type FROM works
    		JOIN variable_storage vs ON works.id = vs.work_id
    		JOIN members m ON vs.id = m.block_id
		 WHERE work_number = $1 `

	if fromActiveNodes {
		q += `AND vs.status IN ('running', 'idle');`
	}

	members := make([]DbMember, 0)

	rows, err := db.Connection.Query(ctx, q, workNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	met := make(map[string]struct{})

	for rows.Next() {
		m := DbMember{}

		if scanErr := rows.Scan(
			&m.Login, &m.Type,
		); scanErr != nil {
			return nil, scanErr
		}

		key := fmt.Sprintf("%s:%s", m.Login, m.Type)
		if _, ok := met[key]; ok {
			continue
		}
		met[key] = struct{}{}

		members = append(members, m)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return members, nil
}

type TaskCustomProps struct {
	IsTest      bool
	CustomTitle string
}

func (db *PGCon) GetTaskCustomProps(ctx c.Context, taskID uuid.UUID) (*TaskCustomProps, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_custom_props")
	defer span.End()

	const q = `
		SELECT run_context -> 'initial_application' -> 'is_test_application',
		       CASE 
			    WHEN run_context -> 'initial_application' -> 'custom_title' IS NULL
			        THEN ''
			        ELSE run_context -> 'initial_application' ->> 'custom_title'
				END
		FROM works
		WHERE id = $1`

	var isTest bool
	var customTitle string
	if err := db.Connection.QueryRow(ctx, q, taskID).Scan(&isTest, &customTitle); err != nil {
		return nil, err
	}

	return &TaskCustomProps{
		IsTest:      isTest,
		CustomTitle: customTitle,
	}, nil
}
func (db *PGCon) GetExecutorsFromPrevExecutionBlockRun(ctx c.Context, taskID uuid.UUID, name string) (
	exec map[string]struct{}, err error) {
	ctx, span := trace.StartSpan(ctx, "get_executor_from_prev_block")
	defer span.End()

	q := `
		SELECT  content-> 'State' -> step_name -> 'executors'
		FROM variable_storage
		WHERE work_id = $1 and step_name = $2 order by time desc limit 1`

	var executors map[string]struct{}
	if err = db.Connection.QueryRow(ctx, q, taskID, name).Scan(&executors); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]struct{}{}, nil
		}
		return map[string]struct{}{}, err
	}

	return executors, nil
}

func (db *PGCon) GetExecutorsFromPrevWorkVersionExecutionBlockRun(ctx c.Context, workNumber, name string) (
	exec map[string]struct{}, err error) {
	ctx, span := trace.StartSpan(ctx, "get_executor_from_prev_block")
	defer span.End()

	var executors map[string]struct{}
	q := `
		SELECT  content-> 'State' -> step_name -> 'executors'
		FROM variable_storage
		WHERE work_id = (select id from works where work_number = $1 order by started_at desc limit 1 offset 1)
		and step_name = $2 order by time desc limit 1`

	if err = db.Connection.QueryRow(ctx, q, workNumber, name).Scan(&executors); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]struct{}{}, nil
		}
		return map[string]struct{}{}, err
	}
	return executors, nil
}
