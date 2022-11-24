package db

import (
	c "context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/lib/pq"
	"time"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func uniqueActionsByRole(login, stepType string, finished bool) string {
	statuses := "('running', 'idle', 'ready')"
	if finished {
		statuses = "('finished', 'no_success')"
	}
	return fmt.Sprintf(`WITH actions AS (
    SELECT vs.work_id                                                                      AS work_id
         , CASE WHEN vs.status = 'running' AND NOT m.finished THEN m.actions ELSE '{}' END AS action
    FROM members m
             JOIN variable_storage vs on vs.id = m.block_id
             JOIN works w on vs.work_id = w.id
    WHERE m.login = '%s'
      AND vs.step_type = '%s'
      AND vs.status IN %s
),
     unique_actions AS (
         SELECT actions.work_id AS work_id, ARRAY_AGG(DISTINCT _unnested.action) AS actions
         FROM actions
                  LEFT JOIN LATERAL (SELECT UNNEST(actions.action) as action) _unnested ON TRUE
         GROUP BY actions.work_id
     )`, login, stepType, statuses)
}

func uniqueActiveActions(login, workNumber string) string {
	return fmt.Sprintf(`WITH actions AS (
    SELECT vs.work_id                                                                      AS work_id
         , CASE WHEN vs.status = 'running' AND NOT m.finished THEN m.actions ELSE '{}' END AS action
    FROM members m
             JOIN variable_storage vs on vs.id = m.block_id
             JOIN works w on vs.work_id = w.id
    WHERE m.login = '%s'
      AND w.work_number = '%s'
      AND vs.status IN ('running', 'idle', 'ready')
),
     unique_actions AS (
         SELECT actions.work_id AS work_id, ARRAY_AGG(DISTINCT _unnested.action) AS actions
         FROM actions
                  LEFT JOIN LATERAL (SELECT UNNEST(actions.action) as action) _unnested ON TRUE
         GROUP BY actions.work_id
     )`, login, workNumber)
}

func getUniqueActions(as, login string) string {
	switch as {
	case "approver":
		return uniqueActionsByRole(login, "approver", false)
	case "finished_approver":
		return uniqueActionsByRole(login, "approver", true)
	case "executor":
		return uniqueActionsByRole(login, "execution", false)
	case "finished_executor":
		return uniqueActionsByRole(login, "execution", true)
	case "form_executor":
		return uniqueActionsByRole(login, "form", false)
	case "finished_form_executor":
		return uniqueActionsByRole(login, "form", true)
	default:
		return fmt.Sprintf(`WITH unique_actions AS (
    SELECT id AS work_id, '{}' AS actions
    FROM works
    WHERE author = '%s'
)`, login)
	}
}

//nolint:gocritic,gocyclo //filters
func compileGetTasksQuery(filters entity.TaskFilter) (q string, args []interface{}) {
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
			p.name,
			COALESCE(descr.description, ''),
			COALESCE(descr.blueprint_id, ''),
			count(*) over() as total,
			w.rate,
			w.rate_comment,
		    CASE WHEN ua.actions <> '{null}' THEN ua.actions ELSE '{}' END
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN pipelines p ON p.id = v.pipeline_id
		JOIN work_status ws ON w.status = ws.id
		JOIN unique_actions ua ON ua.work_id = w.id
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
	if filters.Order != nil {
		order = *filters.Order
	}

	if filters.SelectAs != nil {
		q = fmt.Sprintf("%s %s", getUniqueActions(*filters.SelectAs, filters.CurrentUser), q)
	} else {
		q = fmt.Sprintf("%s %s", getUniqueActions("", filters.CurrentUser), q)
	}

	if filters.TaskIDs != nil {
		args = append(args, filters.TaskIDs)
		q = fmt.Sprintf("%s AND w.work_number = ANY($%d)", q, len(args))
	}
	if filters.Name != nil {
		args = append(args, *filters.Name)
		q = fmt.Sprintf("%s AND p.name ILIKE $%d || '%%'", q, len(args))
	}
	if filters.Created != nil {
		args = append(args, time.Unix(int64(filters.Created.Start), 0).UTC(), time.Unix(int64(filters.Created.End), 0).UTC())
		q = fmt.Sprintf("%s AND w.started_at BETWEEN $%d AND $%d", q, len(args)-1, len(args))
	}
	if filters.Archived != nil {
		switch *filters.Archived {
		case true:
			q = fmt.Sprintf("%s AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) > '3 days'", q)
		case false:
			q = fmt.Sprintf("%s AND ((now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days' OR w.finished_at IS NULL)", q)
		}
	}

	if filters.ForCarousel != nil && *filters.ForCarousel {
		q = fmt.Sprintf("%s AND ((w.human_status='done' AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days')", q)
		q = fmt.Sprintf("%s OR w.human_status = 'wait')", q)
	}

	if filters.Status != nil {
		q = fmt.Sprintf("%s AND (w.human_status IN (%s))", q, *filters.Status)
	}

	if filters.Receiver != nil {
		args = append(args, *filters.Receiver)
		q = fmt.Sprintf("%s AND w.author=$%d ", q, len(args))
	}

	if order != "" {
		q = fmt.Sprintf("%s\n ORDER BY w.started_at %s", q, order)
	}

	if filters.Offset != nil {
		args = append(args, *filters.Offset)
		q = fmt.Sprintf("%s\n OFFSET $%d", q, len(args))
	}

	if filters.Limit != nil {
		args = append(args, *filters.Limit)
		q = fmt.Sprintf("%s\n LIMIT $%d", q, len(args))
	}

	return q, args
}

func (db *PGCon) GetAdditionalForms(workNumber, nodeName string) ([]string, error) {
	const q = `
	WITH content as (
		SELECT jsonb_array_elements(content -> 'State' -> $3 -> 'forms_accessibility') as rules
		FROM variable_storage
			WHERE work_id IN (SELECT id  FROM works WHERE work_number = $1)
		LIMIT 1
	)
	SELECT content -> 'State' -> step_name ->> 'description'
	FROM variable_storage
		WHERE step_name in (
			SELECT rules ->> 'node_id' as rule
			FROM content
			WHERE rules ->> 'accessType' != 'None'
			LIMIT 1
		)
		AND work_id IN (SELECT id FROM works WHERE work_number = $2)
	ORDER BY time`

	ff := make([]string, 0)
	rows, err := db.Connection.Query(c.Background(), q, workNumber, workNumber, nodeName)
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

func (db *PGCon) GetApplicationData(workNumber string) (*orderedmap.OrderedMap, error) {
	const q = `
	SELECT content->'State'->'servicedesk_application_0'
		from variable_storage 
	where step_type = 'servicedesk_application' 
	and work_id = (select id from works where work_number = $1)`

	var data *orderedmap.OrderedMap
	if err := db.Connection.QueryRow(c.Background(), q, workNumber).Scan(&data); err != nil {
		return nil, err
	}
	return data, nil
}

//nolint:gocritic //filters
func (db *PGCon) GetTasks(ctx c.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	ctx, span := trace.StartSpan(ctx, "db.pg_get_tasks")
	defer span.End()

	q, args := compileGetTasksQuery(filters)

	tasks, err := db.getTasks(ctx, q, args)
	if err != nil {
		return nil, err
	}

	total := 0
	if len(tasks.Tasks) > 0 {
		total = tasks.Tasks[0].Total
	}

	return &entity.EriusTasksPage{
		Tasks: tasks.Tasks,
		Total: total,
	}, nil
}

func (db *PGCon) GetTasksCount(ctx c.Context, userName string) (*entity.CountTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks_count")
	defer span.End()
	// nolint:gocritic
	// language=PostgreSQL
	q := fmt.Sprintf(`
		WITH workers as (
			SELECT id, author FROM works
		WHERE works.child_id IS NULL AND status = 1
		)
		SELECT
		(SELECT count(*) FROM workers WHERE workers.author = '%s'),
		(SELECT count(*)
			FROM members m
				JOIN variable_storage vs on vs.id = m.block_id
			WHERE vs.status IN ('running', 'idle', 'ready') AND
				m.login = '%s' AND vs.step_type = 'approver'
		),
		(SELECT count(*)
			 FROM members m
				JOIN variable_storage vs on vs.id = m.block_id
			 WHERE vs.status IN ('running', 'idle', 'ready') AND
				m.login = '%s' AND vs.step_type = 'execution'),
		
		(SELECT count(*)
			FROM members m
				JOIN variable_storage vs on vs.id = m.block_id
			WHERE vs.status IN ('running', 'idle', 'ready') AND
				m.login = '%s' AND vs.step_type = 'form'
		)`, userName, userName, userName, userName)

	counter, err := db.getTasksCount(ctx, q)
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
		WHERE p.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(ctx, q, []interface{}{pipelineID})
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
		WHERE v.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(ctx, q, []interface{}{versionID})
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

func (db *PGCon) GetTask(ctx c.Context, username, workNumber string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := uniqueActiveActions(username, workNumber)

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
			COALESCE(descr.description, ''),
			COALESCE(descr.blueprint_id, ''),
			w.rate,
			w.rate_comment,
         	CASE WHEN ua.actions <> '{null}' THEN ua.actions ELSE '{}' END
		FROM works w 
		JOIN versions v ON v.id = w.version_id
		JOIN pipelines p ON p.id = v.pipeline_id
		JOIN work_status ws ON w.status = ws.id
		JOIN unique_actions ua ON ua.work_id = w.id
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

	return db.getTask(ctx, q, workNumber)
}

func (db *PGCon) getTask(ctx c.Context, q, workNumber string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_private")
	defer span.End()

	et := entity.EriusTask{}

	var nullStringParameters sql.NullString
	var actions pq.StringArray

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
		&actions,
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

	return &et, nil
}

type tasksCounter struct {
	totalActive       int
	totalExecutor     int
	totalApprover     int
	totalFormExecutor int
}

func (db *PGCon) getTasksCount(ctx c.Context, q string) (*tasksCounter, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks_count")
	defer span.End()

	counter := &tasksCounter{}

	if scanErr := db.Connection.QueryRow(ctx, q).
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
func (db *PGCon) getTasks(ctx c.Context, q string, args []interface{}) (*entity.EriusTasks, error) {
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

	for rows.Next() {
		et := entity.EriusTask{}

		var nullStringParameters sql.NullString
		var actions pq.StringArray

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
			&actions,
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

func (db *PGCon) GetUsersWithReadWriteFormAccess(ctx c.Context, workNumber, stepName string) ([]entity.UsersWithFormAccess, error) {
	const q =
	// nolint:gocritic
	// language=PostgreSQL
	`
	with blocks_executors_pair as (
		select
			   content -> 'pipeline' -> 'blocks' -> block_name -> 'params' ->> executor_group_param as executors_group_id,
			   content -> 'pipeline' -> 'blocks' -> block_name -> 'params' ->> 'type' as execution_type,
			   content -> 'pipeline' -> 'blocks' -> block_name -> 'params' ->> executor_param as executor,
			   executor_param,
			   jsonb_array_elements(content -> 'pipeline' -> 'blocks' -> block_name -> 'params' ->
						'forms_accessibility') as access_params
		from (
			with executor_approver_blocks as (
			select content,
				jsonb_object_keys(content -> 'pipeline' -> 'blocks') as block_name
			from versions v
				left join works w on v.id = w.version_id
			where w.work_number = $1
			)
			select
				content,
				block_name,
				case 
				    when block_name like 'approver%' then 'approver' 
				    when block_name like 'execution%' then 'executors' end as executor_param,
				case 
				    when block_name like 'approver%' then 'approvers_group_id' 
				    when block_name like 'execution%' then 'executors_group_id' 
				    end as executor_group_param
			from executor_approver_blocks
			where
				  block_name like 'execution%'
			   or block_name like 'approver%'
		) as result
	)

	select
		case when execution_type = 'fromSchema' then 'from_schema' else execution_type end,
		case when executor_param = 'executors' then 'execution' else executor_param end as block_type,
		executors_group_id,
		executor
	
	from blocks_executors_pair
	where access_params ->> 'accessType' = 'ReadWrite'
	and access_params ->> 'node_id' = $2
	`

	result := make([]entity.UsersWithFormAccess, 0)
	rows, err := db.Connection.Query(ctx, q, workNumber, stepName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		s := entity.UsersWithFormAccess{}

		if err := rows.Scan(
			&s.ExecutionType,
			&s.BlockType,
			&s.GroupId,
			&s.Executor,
		); err != nil {
			return nil, err
		}

		result = append(result, s)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return result, nil
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
