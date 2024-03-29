package db

import (
	c "context"

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

func (db *PGCon) GetTaskForMonitoring(ctx c.Context, workNumber string) ([]e.MonitoringTaskNode, error) {
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
		from works w
    		join versions v on w.version_id = v.id
    		join pipelines p on v.pipeline_id = p.id
    		join variable_storage vs on w.id = vs.work_id
		where w.work_number = $1
		order by vs.time`

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
