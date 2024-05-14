package db

import (
	c "context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"go.opencensus.io/trace"
)

func getTasksForMonitoringQuery(filters *e.TasksForMonitoringFilters) *string {
	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT 	
		    w.id,
		    CASE
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
			w.finished_at AS finished_at,
			p.deleted_at AS process_deleted_at,
			COUNT(*) OVER() AS total
		FROM works w
		LEFT JOIN versions v ON w.version_id = v.id
		LEFT JOIN pipelines p ON v.pipeline_id = p.id
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

	escapeFilter := strings.ReplaceAll(*filter, "_", "!_")
	escapeFilter = strings.ReplaceAll(escapeFilter, "%", "!%")

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

func (db *PGCon) GetTasksForMonitoring(ctx c.Context, dto *e.TasksForMonitoringFilters) (*e.TasksForMonitoring, error) {
	ctx, span := trace.StartSpan(ctx, "get_tasks_for_monitoring")
	defer span.End()

	q := getTasksForMonitoringQuery(dto)

	rows, err := db.Connection.Query(ctx, *q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasksForMonitoring := &e.TasksForMonitoring{
		Tasks: make([]e.TaskForMonitoring, 0),
	}

	var workID uuid.UUID

	for rows.Next() {
		task := e.TaskForMonitoring{}

		err = rows.Scan(
			&workID,
			&task.Status,
			&task.ProcessName,
			&task.Initiator,
			&task.WorkNumber,
			&task.StartedAt,
			&task.FinishedAt,
			&task.ProcessDeletedAt,
			&tasksForMonitoring.Total,
		)
		if err != nil {
			return nil, err
		}

		eventType, eventTime, eventErr := db.getLastEventForMonitoringByWorkID(ctx, workID)
		if eventErr != nil {
			return nil, eventErr
		}

		if eventType == nil {
			et := "start"
			eventType = &et
			eventTime = &task.StartedAt
		}

		task.LastEventType, task.LastEventAt = eventType, eventTime

		tasksForMonitoring.Tasks = append(tasksForMonitoring.Tasks, task)
	}

	return tasksForMonitoring, nil
}

func (db *PGCon) GetTaskForMonitoring(ctx c.Context, workNumber string, fromEventID, toEventID *string) ([]e.MonitoringTaskStep, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_for_monitoring")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT w.status,
			   w.id as work_id,
			   w.work_number,
			   w.version_id,
			   w.is_paused task_is_paused,
			   p.author,
			   p.created_at::text,
			   p.name,
			   vs.step_name,
			   vs.status,
			   vs.id,
			   v.content->'pipeline'-> 'blocks'->step_name->>'title' title,
			   vs.time block_date_init,
			   vs.is_paused block_is_paused
		FROM works w
			 JOIN versions v ON w.version_id = v.id
			 JOIN pipelines p ON v.pipeline_id = p.id
			 JOIN variable_storage vs ON w.id = vs.work_id
		WHERE w.work_number = $1`

	var withSteps string

	filterFromEvent := fromEventID != nil && *fromEventID != ""

	if filterFromEvent {
		withSteps = fmt.Sprintf(`WITH steps AS (
			SELECT vs.id, vs.step_name, vs.time
			FROM variable_storage vs
			LEFT JOIN works w ON w.id = vs.work_id
			WHERE w.work_number = '%s' AND vs.id::text IN (SELECT jsonb_array_elements_text(params -> 'steps')
				FROM task_events WHERE id = '%s')
			ORDER BY vs.time DESC
		)`, workNumber, *fromEventID)
	}

	if filterFromEvent && toEventID != nil && *toEventID != "" {
		q = fmt.Sprintf("%s %s %s", withSteps, q,
			fmt.Sprintf(`AND
			   (
				   (
					   vs.time >= (SELECT created_at FROM task_events WHERE id = '%s') AND
					   vs.time <= (SELECT created_at FROM task_events WHERE id = '%s')
				   ) OR
				   (
					   vs.time =
					   (SELECT time FROM steps
						  WHERE
							step_name = vs.step_name AND
							time < (SELECT created_at FROM task_events WHERE id = '%s')
						  LIMIT 1
					   )
				   )
			   )`,
				*fromEventID,
				*toEventID,
				*fromEventID,
			),
		)
	}

	if filterFromEvent && (toEventID == nil || *toEventID == "") {
		q = fmt.Sprintf("%s %s %s", withSteps, q,
			fmt.Sprintf(`AND
			   (
					vs.time >= (SELECT created_at FROM task_events WHERE id = '%s') OR
				   (
					   vs.time =
					   (SELECT time FROM steps
						  WHERE
							step_name = vs.step_name AND
							time < (SELECT created_at FROM task_events WHERE id = '%s')
						  LIMIT 1
					   )
				   )
			   )`, *fromEventID, *fromEventID),
		)
	}

	q = fmt.Sprintf("%s %s", q, "ORDER BY vs.time")

	rows, err := db.Connection.Query(ctx, q, workNumber)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := make([]e.MonitoringTaskStep, 0)

	for rows.Next() {
		item := e.MonitoringTaskStep{}
		if scanErr := rows.Scan(
			&item.WorkStatus,
			&item.WorkID,
			&item.WorkNumber,
			&item.VersionID,
			&item.IsPaused,
			&item.Author,
			&item.CreationTime,
			&item.ScenarioName,
			&item.NodeID,
			&item.Status,
			&item.BlockID,
			&item.RealName,
			&item.BlockDateInit,
			&item.BlockIsPaused,
		); scanErr != nil {
			return nil, scanErr
		}

		res = append(res, item)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return res, nil
}

func (db *PGCon) getLastEventForMonitoringByWorkID(ctx c.Context, workID uuid.UUID) (eventType *string, eventTime *time.Time, err error) {
	ctx, span := trace.StartSpan(ctx, "get_task_for_monitoring")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
			SELECT created_at, event_type 
			FROM task_events WHERE work_id = $1 
			ORDER BY created_at DESC LIMIT 1
		`

	row := db.Connection.QueryRow(ctx, q, workID)

	var eType *string

	var t *time.Time

	err = row.Scan(&t, &eType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}

		return nil, nil, err
	}

	return eType, t, nil
}
