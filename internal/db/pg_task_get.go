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

	"github.com/lib/pq"

	"github.com/pkg/errors"

	"golang.org/x/exp/slices"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

func uniqueActionsByRole(loginsIn, stepType string, finished bool) string {
	statuses := "('running', 'idle', 'ready')"
	if finished {
		statuses = "('finished', 'no_success')"
	}
	return fmt.Sprintf(`WITH actions AS (
    SELECT vs.work_id                                                                      AS work_id
         , CASE WHEN vs.status IN ('running', 'idle') AND NOT m.finished THEN m.actions ELSE '{}' END AS action
    FROM members m
             JOIN variable_storage vs on vs.id = m.block_id
             JOIN works w on vs.work_id = w.id
    WHERE m.login IN %s
      AND vs.step_type = '%s'
      AND vs.status IN %s
      AND w.child_id IS NULL
),
     unique_actions AS (
         SELECT actions.work_id AS work_id, ARRAY_AGG(DISTINCT _unnested.action) AS actions
         FROM actions
                  LEFT JOIN LATERAL (SELECT UNNEST(actions.action) as action) _unnested ON TRUE
         GROUP BY actions.work_id
     )`, loginsIn, stepType, statuses)
}

func uniqueActiveActions(approverLogins, executionLogins []string, currentUser, workNumber string) string {
	var approverLoginsIn = buildInExpression(approverLogins)
	var executionLoginsIn = buildInExpression(executionLogins)

	return fmt.Sprintf(`WITH actions AS (
    SELECT vs.work_id                                                                      AS work_id
         , CASE WHEN vs.status IN ('running', 'idle') AND NOT m.finished THEN m.actions ELSE '{}' END AS action
    FROM members m
             JOIN variable_storage vs on vs.id = m.block_id
             JOIN works w on vs.work_id = w.id
    WHERE (m.login = '%s' AND vs.step_type = 'form')
       OR (m.login IN %s AND vs.step_type = 'approver')
       OR (m.login IN %s AND vs.step_type = 'execution')
      AND w.work_number = '%s'
      AND vs.status IN ('running', 'idle', 'ready')
	  AND w.child_id IS NULL
	),
     unique_actions AS (
         SELECT actions.work_id AS work_id, ARRAY_AGG(DISTINCT _unnested.action) AS actions
         FROM actions
                  LEFT JOIN LATERAL (SELECT UNNEST(actions.action) as action) _unnested ON TRUE
         GROUP BY actions.work_id
     )`, currentUser, approverLoginsIn, executionLoginsIn, workNumber)
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

func getUniqueActions(as string, logins []string) string {
	var loginsIn = buildInExpression(logins)

	switch as {
	case "approver":
		return uniqueActionsByRole(loginsIn, "approver", false)
	case "finished_approver":
		return uniqueActionsByRole(loginsIn, "approver", true)
	case "executor":
		return uniqueActionsByRole(loginsIn, "execution", false)
	case "finished_executor":
		return uniqueActionsByRole(loginsIn, "execution", true)
	case "form_executor":
		return uniqueActionsByRole(loginsIn, "form", false)
	case "finished_form_executor":
		return uniqueActionsByRole(loginsIn, "form", true)
	case "initiators":
		return fmt.Sprintf(`WITH unique_actions AS (
			SELECT id AS work_id, '{}' AS actions
			FROM works
			WHERE status = 1 AND author IN %s AND child_id IS NULL
		)`, loginsIn)
	default:
		return fmt.Sprintf(`WITH unique_actions AS (
    SELECT id AS work_id, '{}' AS actions
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
			CASE
        		WHEN w.run_context -> 'initial_application' -> 'is_test_application' = 'true'
            		THEN concat(p.name, ' (ТЕСТОВАЯ ЗАЯВКА)')
        		ELSE p.name
    		END,
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
		q = fmt.Sprintf(getUniqueActions(*fl.SelectAs, delegations), q)
	} else {
		q = fmt.Sprintf("%s %s", getUniqueActions("", delegations), q)
	}

	if fl.TaskIDs != nil {
		args = append(args, fl.TaskIDs)
		q = fmt.Sprintf("%s AND w.work_number = ANY($%d)", q, len(args))
	}
	if fl.Name != nil {
		name := strings.Replace(*fl.Name, "_", "!_", -1)
		name = strings.Replace(name, "%", "!%", -1)
		args = append(args, name)
		q = fmt.Sprintf("%s AND p.name ILIKE $%d ESCAPE '!' || '%%'", q, len(args))
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

	varStorage := getProcessingSteps(&fl)
	if varStorage != "" {
		q = fmt.Sprintf("%s %s", varStorage, q)
		q = fmt.Sprintf("%s AND w.status = 1 AND w.id = ANY($%d)", q, len(args))
		q = strings.Replace(q, "[join_variable_storage]", "JOIN var_storage vs ON vs.work_id = w.id", 1)
	} else {
		q = strings.Replace(q, "[join_variable_storage]", "", 1)
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

	return q, args
}

func getProcessingSteps(fl *entity.TaskFilter) string {
	if fl == nil {
		return ""
	}

	if fl.ProcessingLogins == nil && fl.ProcessingGroupIds == nil && fl.ExecutorTypeAssigned == nil {
		return ""
	}

	q := `WITH var_storage as (
		SELECT work_id FROM variable_storage 
		WHERE work_id IS NOT NULL AND status = 'running'
	)`

	q = addAssignType(q, fl.CurrentUser, fl.ExecutorTypeAssigned)
	q = addProcessingLogins(q, fl.ProcessingLogins)
	q = addProcessingGroups(q, fl.ProcessingGroupIds)

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

func addProcessingLogins(q string, logins *[]string) string {
	if logins == nil || len(*logins) == 0 {
		return q
	}

	return fmt.Sprintf(`%s AND step_type IN('execution', 'approver', 'form') AND 
		(
			(step_type = 'approver' AND content -> 'State' -> step_name -> 'approvers' = ANY %s) OR
			(step_type = 'execution' AND content -> 'State' -> step_name -> 'executors' = ANY %s) OR
			(step_type = 'form' AND content -> 'State' -> step_name -> 'executors' = ANY %s)
		)`, q, *logins, *logins, *logins)
}

func addProcessingGroups(q string, groupIds *[]string) string {
	if groupIds == nil || len(*groupIds) == 0 {
		return q
	}

	return fmt.Sprintf(`%s AND step_type IN('execution', 'approver') 
		AND (
			(step_type = 'execution' AND content -> 'State' -> step_name -> 'executors_group_id' = ANY %s) OR 
			(step_type = 'approver' AND content -> 'State' -> step_name -> 'approvers_group_id' = ANY %s)
		)`,
		q,
		*groupIds,
		*groupIds,
	)
}

func (db *PGCon) GetAdditionalForms(workNumber, nodeName string) ([]string, error) {
	const q = `
	WITH content as (
		SELECT jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> $2 -> 'params' -> 'forms_accessibility') as rules
		FROM versions
			WHERE id = (SELECT version_id FROM works WHERE work_number = $1 AND child_id IS NULL)
	)
    SELECT content -> 'State' -> step_name ->> 'description'
	FROM variable_storage
		WHERE step_name in (
			SELECT rules ->> 'node_id' as rule
			FROM content
			WHERE rules ->> 'accessType' != 'None'
		)
		AND work_id = (SELECT id FROM works WHERE work_number = $1 AND child_id IS NULL)
	ORDER BY time`
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
	for _, task := range tasks.Tasks {
		taskIDs = append(taskIDs, task.ID.String())
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
			  WHERE w.id = ANY($1)) blocks_with_work_id
		WHERE key(blocks) NOT LIKE 'form%%'
		   OR (
					key(blocks) LIKE 'form%%'
				AND value(blocks) ->> 'executors' SIMILAR TO '{"(%s)": {}}'
			)
	), data AS (SELECT work_id,
					   jsonb_each(blocks -> 'application_body')                           AS form_and_sd_application_body,
					   jsonb_array_elements(blocks -> 'additional_info') -> 'attachments' AS additional_info_attachments,
					   jsonb_array_elements(blocks -> 'approver_log') -> 'attachments'    AS approver_log_attachments,
					   jsonb_array_elements(blocks -> 'editing_app_log') -> 'attachments' AS editing_app_log_attachments
				FROM blocks_with_filtered_forms),
		 counts AS (SELECT
						work_id,
						COUNT(form_and_sd_application_body) AS form_and_sd_count,
						SUM(coalesce(jsonb_array_length(NULLIF(additional_info_attachments, 'null')), 0)) AS additional_attachment_count,
						SUM(coalesce(jsonb_array_length(NULLIF(approver_log_attachments, 'null')), 0)) AS additional_approvers_count,
						SUM(coalesce(jsonb_array_length(NULLIF(editing_app_log_attachments, 'null')), 0)) AS rework_count
					FROM data
					WHERE value(form_and_sd_application_body)::text LIKE '"attachment:%%'
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
		WITH ids AS (
		    SELECT w.id
		    FROM works w
         	JOIN versions v ON v.id = w.version_id
         	JOIN pipelines p ON p.id = v.pipeline_id
         	JOIN work_status ws ON w.status = ws.id
			WHERE w.child_id IS NULL
		)
		SELECT
		(SELECT count(*) FROM works w join ids on w.id = ids.id
		WHERE author = $1 AND (w.finished_at IS NULL OR (w.archived = false AND
		      (now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days'))),
		(SELECT count(*)
			FROM members m
				JOIN variable_storage vs on vs.id = m.block_id
				JOIN ids on vs.work_id = ids.id
			WHERE vs.status IN ('running', 'idle', 'ready') AND
				m.login = ANY($2) AND vs.step_type = 'approver'
		),
		(SELECT count(*)
			 FROM members m
				JOIN variable_storage vs on vs.id = m.block_id
				JOIN ids on vs.work_id = ids.id
			 WHERE vs.status IN ('running', 'idle', 'ready') AND
				m.login = ANY($3) AND vs.step_type = 'execution'),
		
		(SELECT count(*)
			FROM members m
				JOIN variable_storage vs on vs.id = m.block_id
				JOIN ids on vs.work_id = ids.id
			WHERE vs.status IN ('running', 'idle', 'ready') AND
				m.login = $1 AND vs.step_type = 'form'
		)`

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
			 CASE
        		WHEN run_context -> 'initial_application' -> 'is_test_application' = 'true'
            		THEN concat(p.name, ' (ТЕСТОВАЯ ЗАЯВКА)')
        		ELSE p.name
    		END,
			COALESCE(w.run_context -> 'initial_application' ->> 'description',
                COALESCE(descr.description, '')),
			COALESCE(descr.blueprint_id, ''),
			w.rate,
			w.rate_comment,
         	ua.actions
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
	var nullStringActions []sql.NullString

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
		&et.Description,
		&et.BlueprintID,
		&et.Rate,
		&et.RateComment,
		pq.Array(&nullStringActions),
	)
	if err != nil {
		return nil, err
	}

	actions := db.actionsToStrings(nullStringActions)

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

func (db *PGCon) computeActions(ctx c.Context, currentUserDelegators, actions []string,
	allActions map[string]entity.TaskAction, author string) (result []entity.TaskAction, err error) {
	const (
		CancelAppId       = "cancel_app"
		CancelAppPriority = "other"
		CancelAppTitle    = "Отозвать"
	)

	var computedActions = make([]entity.TaskAction, 0)
	var computedActionIds = make([]string, 0)
	var actionsToIgnore = getActionsToIgnoreIfOtherExist()

	result = make([]entity.TaskAction, 0)

	for _, actionId := range actions {
		var compositeActionId = strings.Split(actionId, ":")
		if len(compositeActionId) > 1 {
			var id = compositeActionId[0]
			var priority = compositeActionId[1]
			var actionWithPreferences = allActions[id]

			var computedAction = entity.TaskAction{
				Id:                 id,
				ButtonType:         priority,
				Title:              actionWithPreferences.Title,
				CommentEnabled:     actionWithPreferences.CommentEnabled,
				AttachmentsEnabled: actionWithPreferences.AttachmentsEnabled,
				IsPublic:           actionWithPreferences.IsPublic,
			}

			computedActions = append(computedActions, computedAction)
			computedActionIds = append(computedActionIds, computedAction.Id)
		}
	}

	for _, a := range computedActions {
		var ignoreAction = false

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

	var isDelegateOfAuthor = slices.Contains(currentUserDelegators, author)

	if ui.Username == author || isDelegateOfAuthor {
		var cancelAppAction = entity.TaskAction{
			Id:                 CancelAppId,
			ButtonType:         CancelAppPriority,
			Title:              CancelAppTitle,
			CommentEnabled:     true,
			AttachmentsEnabled: false,
		}

		result = append(result, cancelAppAction)
	}

	return result, nil
}

type tasksCounter struct {
	totalActive       int
	totalExecutor     int
	totalApprover     int
	totalFormExecutor int
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
		var nullStringActions []sql.NullString

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
			&et.Description,
			&et.BlueprintID,
			&et.Total,
			&et.Rate,
			&et.RateComment,
			pq.Array(&nullStringActions),
		)

		if err != nil {
			return nil, err
		}

		if nullStringParameters.Valid && nullStringParameters.String != "" {
			err = json.Unmarshal([]byte(nullStringParameters.String), &et.Parameters)
			if err != nil {
				return nil, err
			}
		}

		actions := db.actionsToStrings(nullStringActions)
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
			WHERE work_id = $1 AND vs.status != 'skipped'
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

func (db *PGCon) getActionsMap(ctx c.Context) (actions map[string]entity.TaskAction, err error) {
	const q = `
		SELECT 
			id,
			title,
			is_public,
			comment_enabled,
			attachments_enabled
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
		WHERE cnt >= 30
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
		rows.Close()
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

	q := fmt.Sprintf(`SELECT jsonb_merge_agg(vs.content) as content FROM variable_storage vs
    	WHERE work_id = '%s' AND step_name IN %s`, workId, buildInExpression(blockIds))

	var content []byte
	if err := db.Connection.QueryRow(ctx, q).Scan(&content); err != nil {
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
						WHEN w.status = 2 THEN 'Завершен' WHEN w.status = 4 THEN 'Остановлен' 
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
			WHERE w.started_at IS NOT NULL AND p.name IS NOT NULL
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
