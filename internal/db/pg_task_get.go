package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

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
		    COALESCE(descr.blueprint_id, '')
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		LEFT JOIN LATERAL (
             SELECT * FROM pipeliner.variable_storage vs
             WHERE vs.work_id = w.id AND vs.status != 'skipped'
             ORDER BY vs.time DESC
             --limit--
        ) workers ON workers.work_id = w.id
		LEFT JOIN LATERAL (
		    SELECT work_id, 
		           content::json->'State'->step_name->>'description' description,
		           content::json->'State'->step_name->>'blueprint_id' blueprint_id
		    FROM pipeliner.variable_storage vs
		    WHERE vs.work_id = w.id AND vs.step_type = 'servicedesk_application' AND vs.status != 'skipped'
		    ORDER BY vs.time DESC
		    LIMIT 1
		) descr ON descr.work_id = w.id
		WHERE w.child_id IS NULL`

	order := "ASC"
	if filters.Order != nil {
		order = *filters.Order
	}

	args = append(args, filters.CurrentUser)
	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case "approver":
			{
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'approvers'->$%d "+
					"IS NOT NULL AND workers.status IN ('running', 'idle', 'ready')", q, len(args))
			}
		case "finished_approver":
			{
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'approvers'->$%d "+
					"IS NOT NULL AND workers.status IN ('finished', 'no_success')", q, len(args))
			}
		case "executor":
			{
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'executors'->$%d "+
					"IS NOT NULL AND (workers.status IN ('running', 'idle', 'ready'))", q, len(args))
			}
		case "finished_executor":
			{
				q = fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'executors'->$%d "+
					"IS NOT NULL AND (workers.status IN ('finished', 'no_success'))", q, len(args))
			}
		}
	} else {
		q = fmt.Sprintf("%s AND w.author = $%d", q, len(args))
		q = strings.Replace(q, "--limit--", "LIMIT 1", -1)
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

//nolint:gocritic //filters
func (db *PGCon) GetTasks(c context.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks")
	defer span.End()

	q, args := compileGetTasksQuery(filters)

	tasks, err := db.getTasks(c, q, args)
	if err != nil {
		return nil, err
	}

	filters.Limit = nil
	filters.Offset = nil
	q, args = compileGetTasksQuery(filters)

	count, err := db.getTasksCount(c, q, args)
	if err != nil {
		return nil, err
	}

	return &entity.EriusTasksPage{
		Tasks: tasks.Tasks,
		Total: count,
	}, nil
}

func (db *PGCon) GetTasksCount(c context.Context, userName string) (*entity.CountTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks_count")
	defer span.End()
	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT 
			w.id
		FROM pipeliner.works w 
		JOIN pipeliner.work_status ws ON w.status = ws.id
		LEFT JOIN LATERAL (
             SELECT * FROM pipeliner.variable_storage vs
             WHERE vs.work_id = w.id AND vs.status != 'skipped'
             ORDER BY vs.time DESC
             --limit--
        ) workers ON workers.work_id = w.id
		WHERE 1=1`

	var args []interface{}
	qActive := fmt.Sprintf("%s AND w.author = '%s'", q, userName)
	qActive = strings.Replace(qActive, "--limit--", "LIMIT 1", -1)
	active, err := db.getTasksCount(c, qActive, args)
	if err != nil {
		return nil, err
	}

	qApprover := fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'approvers'->'%s' "+
		"IS NOT NULL AND workers.status IN ('running', 'idle', 'ready')", q, userName)
	approver, err := db.getTasksCount(c, qApprover, args)
	if err != nil {
		return nil, err
	}

	qExecutor := fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'executors'->'%s' "+
		"IS NOT NULL AND (workers.status IN ('running', 'idle', 'ready'))", q, userName)
	executor, err := db.getTasksCount(c, qExecutor, args)
	if err != nil {
		return nil, err
	}

	return &entity.CountTasks{
		TotalActive:   active,
		TotalExecutor: executor,
		TotalApprover: approver,
	}, nil
}

func (db *PGCon) GetPipelineTasks(c context.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_tasks")
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
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE p.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(c, q, []interface{}{pipelineID})
}

func (db *PGCon) GetVersionTasks(c context.Context, versionID uuid.UUID) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_version_tasks")
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
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		ORDER BY w.started_at DESC
		LIMIT 100`

	return db.getTasks(c, q, []interface{}{versionID})
}

func (db *PGCon) GetLastDebugTask(c context.Context, id uuid.UUID, author string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_last_debug_task")
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
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		AND w.author = $2
		AND w.debug = true
		ORDER BY w.started_at DESC
		LIMIT 1`

	et := entity.EriusTask{}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(c, q, id, author)
	parameters := ""

	err = row.Scan(&et.ID, &et.StartedAt, &et.Status, &et.HumanStatus, &et.IsDebugMode, &parameters, &et.Author, &et.VersionID)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(parameters), &et.Parameters)
	if err != nil {
		return nil, err
	}

	return &et, nil
}

func (db *PGCon) GetTask(c context.Context, workNumber string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_task")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `SELECT 
			w.id, 
			w.started_at, 
			w.started_at, 
			ws.name,
			w.human_status,
			w.debug, 
			COALESCE(w.parameters, '{}') AS parameters,
			w.author,
			w.version_id,
			w.work_number,
			p.name,
			COALESCE(descr.description, ''),
			COALESCE(descr.blueprint_id, '')
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		LEFT JOIN LATERAL (
			SELECT work_id, 
				content::json->'State'->step_name->>'description' description,
				content::json->'State'->step_name->>'blueprint_id' blueprint_id
			FROM pipeliner.variable_storage vs
			WHERE vs.work_id = w.id AND vs.step_type = 'servicedesk_application' AND vs.status != 'skipped'
			ORDER BY vs.time DESC
			LIMIT 1
		) descr ON descr.work_id = w.id
		WHERE w.work_number = $1 
			AND w.child_id IS NULL
`

	return db.getTask(c, q, workNumber)
}

func (db *PGCon) getTask(c context.Context, q, workNumber string) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_get_task_private")
	defer span.End()

	et := entity.EriusTask{}

	var nullStringParameters sql.NullString

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(c, q, workNumber)

	err = row.Scan(
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

func (db *PGCon) getTasksCount(c context.Context, q string, args []interface{}) (int, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks_count")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return -1, err
	}

	defer conn.Release()

	q = fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub", q)

	var count int
	if scanErr := conn.QueryRow(c, q, args...).Scan(&count); scanErr != nil {
		return -1, scanErr
	}
	return count, nil
}

func (db *PGCon) getTasks(c context.Context, q string, args []interface{}) (*entity.EriusTasks, error) {
	c, span := trace.StartSpan(c, "pg_get_tasks")
	defer span.End()

	ets := entity.EriusTasks{
		Tasks: make([]entity.EriusTask, 0),
	}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	rows, err := conn.Query(c, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		et := entity.EriusTask{}

		var nullStringParameters sql.NullString

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

func (db *PGCon) GetTaskSteps(c context.Context, id uuid.UUID) (entity.TaskSteps, error) {
	c, span := trace.StartSpan(c, "pg_get_task_steps")
	defer span.End()

	el := entity.TaskSteps{}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

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
			vs.status
		FROM pipeliner.variable_storage vs 
			WHERE work_id = $1 AND vs.status != 'skipped'
		ORDER BY vs.time DESC`

	rows, err := conn.Query(c, query, id)
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
