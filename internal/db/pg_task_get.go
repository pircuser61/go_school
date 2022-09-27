package db

import (
	c "context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/iancoleman/orderedmap"
	"golang.org/x/net/context"
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
			COALESCE(descr.blueprint_id, ''),
			w.active_blocks,
			w.skipped_blocks,
			w.notified_blocks,
			w.prev_update_status_blocks
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
				q = fmt.Sprintf("%s AND workers.content @? ('$.State.' || workers.step_name || '.approvers.' || $%d )::jsonpath "+
					" AND workers.status IN ('running', 'idle', 'ready')", q, len(args))
			}
		case "finished_approver":
			{
				q = fmt.Sprintf("%s AND workers.content @? ('$.State.' || workers.step_name || '.approvers.' || $%d )::jsonpath "+
					" AND workers.status IN ('finished', 'no_success')", q, len(args))
			}
		case "executor":
			{
				q = fmt.Sprintf("%s AND workers.content @? ('$.State.' || workers.step_name || '.executors.' || $%d )::jsonpath "+
					" AND (workers.status IN ('running', 'idle', 'ready'))", q, len(args))
			}
		case "finished_executor":
			{
				q = fmt.Sprintf("%s AND workers.content @? ('$.State.' || workers.step_name || '.executors.' || $%d )::jsonpath "+
					" AND (workers.status IN ('finished', 'no_success'))", q, len(args))
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

	if filters.ForCarousel != nil && *filters.ForCarousel {
		q = fmt.Sprintf("%s AND ((w.human_status='done' AND (now()::TIMESTAMP - w.finished_at::TIMESTAMP) < '3 days')", q)
		q = fmt.Sprintf("%s OR w.human_status = 'wait')", q)
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

func (db *PGCon) GetApplicationData(workNumber string) (*orderedmap.OrderedMap, error) {
	q := `SELECT content->'State'->'servicedesk_application_0'
from pipeliner.variable_storage 
where step_type = 'servicedesk_application' 
and work_id = (select id from pipeliner.works where work_number = $1)`
	var data *orderedmap.OrderedMap
	if err := db.Pool.QueryRow(context.Background(), q, workNumber).Scan(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (db *PGCon) SetApplicationData(workNumber string, data *orderedmap.OrderedMap) error {
	q := `UPDATE pipeliner.variable_storage 
set content = jsonb_set(content, '{State,servicedesk_application_0}', '%s')
where work_id = (select id from pipeliner.works where work_number = $1) and step_type in ('servicedesk_application', 'execution')`
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	q = fmt.Sprintf(q, string(bytes))
	_, err = db.Pool.Exec(context.Background(), q, workNumber)
	return err
}

func (db *PGCon) GetNotifData(ctx c.Context) ([]entity.NeededNotif, error) {
	ctxLocal, span := trace.StartSpan(ctx, "makeAndSendNotif")
	defer span.End()
	q := `select
    w.work_number,
    w.author,
    vs.content::json -> 'State' -> 'servicedesk_application_0' -> 'application_body' -> 'recipient' ->> 'username',
    vs.content::json -> 'State' -> 'servicedesk_application_0' -> 'application_body',
    w.human_status
from pipeliner.variable_storage vs
         join pipeliner.works w on vs.work_id = w.id
where work_id in (select id from pipeliner.works where version_id = '12ba4306-dec4-4623-9d2d-666326948e0a')
  and step_type = 'servicedesk_application'
  and vs.content::json -> 'State' -> 'servicedesk_application_0' ->> 'application_body' != ''
order by w.started_at asc`
	rows, err := db.Pool.Query(ctxLocal, q)
	if err != nil {
		return nil, err
	}
	res := make([]entity.NeededNotif, 0)

	for rows.Next() {
		var item entity.NeededNotif
		var descr map[string]interface{}
		if err := rows.Scan(&item.WorkNum, &item.Initiator, &item.Recipient, &descr, &item.Status); err != nil {
			return nil, err
		}
		if descr["kolichestvo_visschih_obrazovanii"] == nil {
			var description entity.NotifData1
			bytes, err := json.Marshal(descr)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(bytes, &description); err != nil {
				return nil, err
			}
			item.Description = description
		} else {
			switch descr["kolichestvo_visschih_obrazovanii"].(string) {
			case "5":
				var description entity.NotifData5
				bytes, err := json.Marshal(descr)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(bytes, &description); err != nil {
					return nil, err
				}
				item.Description = description
			case "4":
				var description entity.NotifData4
				bytes, err := json.Marshal(descr)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(bytes, &description); err != nil {
					return nil, err
				}
				item.Description = description
			case "3":
				var description entity.NotifData3
				bytes, err := json.Marshal(descr)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(bytes, &description); err != nil {
					return nil, err
				}
				item.Description = description
			case "2":
				var description entity.NotifData2
				bytes, err := json.Marshal(descr)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(bytes, &description); err != nil {
					return nil, err
				}
				item.Description = description
			default:
				var description entity.NotifData1
				bytes, err := json.Marshal(descr)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(bytes, &description); err != nil {
					return nil, err
				}
				item.Description = description
			}
		}
		res = append(res, item)
	}
	return res, nil
}

//nolint:gocritic //filters
func (db *PGCon) GetTasks(ctx c.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks")
	defer span.End()

	q, args := compileGetTasksQuery(filters)

	tasks, err := db.getTasks(ctx, q, args)
	if err != nil {
		return nil, err
	}

	filters.Limit = nil
	filters.Offset = nil
	emptyOrder := ""
	filters.Order = &emptyOrder
	q, args = compileGetTasksQuery(filters)

	count, err := db.getTasksCount(ctx, q, args)
	if err != nil {
		return nil, err
	}

	return &entity.EriusTasksPage{
		Tasks: tasks.Tasks,
		Total: count,
	}, nil
}

//nolint:gocritic //filters
func (db *PGCon) GetUnfinishedTasks(ctx c.Context) (*entity.EriusTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_unfinished_tasks")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `SELECT 
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
			COALESCE(descr.blueprint_id, ''),
			w.active_blocks,
			w.skipped_blocks,
			w.notified_blocks,
			w.prev_update_status_blocks
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
		WHERE w.status = 1 AND w.child_id IS NULL`

	return db.getTasks(ctx, query, []interface{}{})
}

func (db *PGCon) GetTasksCount(ctx c.Context, userName string) (*entity.CountTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks_count")
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
		WHERE w.child_id IS NULL`

	var args []interface{}
	qActive := fmt.Sprintf("%s AND w.author = '%s'", q, userName)
	qActive = strings.Replace(qActive, "--limit--", "LIMIT 1", -1)
	active, err := db.getTasksCount(ctx, qActive, args)
	if err != nil {
		return nil, err
	}

	qApprover := fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'approvers'->'%s' "+
		"IS NOT NULL AND workers.status IN ('running', 'idle', 'ready')", q, userName)
	approver, err := db.getTasksCount(ctx, qApprover, args)
	if err != nil {
		return nil, err
	}

	qExecutor := fmt.Sprintf("%s AND workers.content::json->'State'->workers.step_name->'executors'->'%s' "+
		"IS NOT NULL AND (workers.status IN ('running', 'idle', 'ready'))", q, userName)
	executor, err := db.getTasksCount(ctx, qExecutor, args)
	if err != nil {
		return nil, err
	}

	return &entity.CountTasks{
		TotalActive:   active,
		TotalExecutor: executor,
		TotalApprover: approver,
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
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.pipelines p ON p.id = v.pipeline_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
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
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
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
		FROM pipeliner.works w 
		JOIN pipeliner.versions v ON v.id = w.version_id
		JOIN pipeliner.work_status ws ON w.status = ws.id
		WHERE v.id = $1
		AND w.author = $2
		AND w.debug = true
		ORDER BY w.started_at DESC
		LIMIT 1`

	et := entity.EriusTask{}

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(ctx, q, id, author)
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

func (db *PGCon) GetTask(ctx c.Context, workNumber string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task")
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

	return db.getTask(ctx, q, workNumber)
}

func (db *PGCon) getTask(ctx c.Context, q, workNumber string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_task_private")
	defer span.End()

	et := entity.EriusTask{}

	var nullStringParameters sql.NullString

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	row := conn.QueryRow(ctx, q, workNumber)

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

func (db *PGCon) getTasksCount(ctx c.Context, q string, args []interface{}) (int, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks_count")
	defer span.End()

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return -1, err
	}

	defer conn.Release()

	q = fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub", q)

	var count int
	if scanErr := conn.QueryRow(ctx, q, args...).Scan(&count); scanErr != nil {
		return -1, scanErr
	}
	return count, nil
}

//nolint:gocyclo //its ok here
func (db *PGCon) getTasks(ctx c.Context, q string, args []interface{}) (*entity.EriusTasks, error) {
	ctx, span := trace.StartSpan(ctx, "pg_get_tasks")
	defer span.End()

	ets := entity.EriusTasks{
		Tasks: make([]entity.EriusTask, 0),
	}

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	rows, err := conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		et := entity.EriusTask{}

		var nullStringParameters sql.NullString
		var nullJsonActiveBlocks sql.NullString
		var nullJsonSkippedBlocks sql.NullString
		var nullJsonNotifiedBlocks sql.NullString
		var nullJsonPrevUpdateStatusBlocks sql.NullString

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
			&nullJsonActiveBlocks,
			&nullJsonSkippedBlocks,
			&nullJsonNotifiedBlocks,
			&nullJsonPrevUpdateStatusBlocks,
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

		if nullJsonActiveBlocks.Valid {
			err = json.Unmarshal([]byte(nullJsonActiveBlocks.String), &et.ActiveBlocks)
			if err != nil {
				return nil, err
			}
		}

		if nullJsonSkippedBlocks.Valid {
			err = json.Unmarshal([]byte(nullJsonSkippedBlocks.String), &et.SkippedBlocks)
			if err != nil {
				return nil, err
			}
		}

		if nullJsonNotifiedBlocks.Valid {
			err = json.Unmarshal([]byte(nullJsonNotifiedBlocks.String), &et.NotifiedBlocks)
			if err != nil {
				return nil, err
			}
		}

		if nullJsonPrevUpdateStatusBlocks.Valid {
			err = json.Unmarshal([]byte(nullJsonPrevUpdateStatusBlocks.String), &et.PrevUpdateStatusBlocks)
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

	conn, err := db.Pool.Acquire(ctx)
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

	rows, err := conn.Query(ctx, query, id)
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
