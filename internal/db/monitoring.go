package db

import (
	c "context"
	"fmt"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"go.opencensus.io/trace"
)

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

	for rows.Next() {
		task := e.TaskForMonitoring{}

		err = rows.Scan(&task.Status,
			&task.ProcessName,
			&task.Initiator,
			&task.WorkNumber,
			&task.StartedAt,
			&task.FinishedAt,
			&task.ProcessDeletedAt,
			&task.LastEventType,
			&task.LastEventAt,
			&tasksForMonitoring.Total,
		)
		if err != nil {
			return nil, err
		}

		tasksForMonitoring.Tasks = append(tasksForMonitoring.Tasks, task)
	}

	return tasksForMonitoring, nil
}

func (db *PGCon) GetTaskForMonitoring(ctx c.Context, workNumber string, fromEventID, ToEventID *string) ([]e.MonitoringTaskNode, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_for_monitoring")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		SELECT w.work_number, 
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

	if fromEventID != nil && *fromEventID != "" && ToEventID != nil && *ToEventID != "" {
		q = fmt.Sprintf("%s %s", q,
			fmt.Sprintf(
				`AND vs.time >= (SELECT created_at FROM task_events WHERE id = %s)
						AND vs.time <= (SELECT created_at FROM task_events WHERE id = %s)`,
				*fromEventID,
				*ToEventID,
			),
		)
	}

	if (fromEventID != nil && *fromEventID != "") && (ToEventID == nil || *ToEventID == "") {
		q = fmt.Sprintf("%s %s", q,
			fmt.Sprintf(`AND vs.time >= (SELECT created_at FROM task_events WHERE id = %s)`, *fromEventID),
		)
	}

	q = fmt.Sprintf("%s %s", q, "ORDER BY vs.time")

	rows, err := db.Connection.Query(ctx, q, workNumber)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := make([]e.MonitoringTaskNode, 0)

	for rows.Next() {
		item := e.MonitoringTaskNode{}
		if scanErr := rows.Scan(
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
