package db

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"go.opencensus.io/trace"
)

const (
	filledType = "filled"
	emptyType  = "empty"
)

func (db *PGCon) GetTasksToRetry(
	ctx c.Context,
	minLifetime, maxLifetime time.Duration,
	limit int,
) (emptyTasks, filledTasks []*Task, err error) {
	ctx, span := trace.StartSpan(ctx, "tasks_to_retry")
	defer span.End()

	const query = `
	SELECT w.id as id, w.version_id, w.work_number, w.author, w.run_context,
	CASE 
		WHEN (w.status = 1) THEN 'filled'
		WHEN (w.status = 5) THEN 'empty'
	END as type
	FROM works w
	INNER JOIN variable_storage vs ON vs.work_id = w.id
	WHERE w.version_id IS NOT NULL AND
	now() - vs.time > interval '1 second' * $2 AND 
	now() - vs.time < interval '1 second' * $3 AND
	NOT w.is_paused AND
	w.status IN (1,5) AND
	vs.status IN ('new','ready')
	ORDER BY vs.time ASC
	LIMIT $1
	`

	rows, err := db.Connection.Query(ctx, query, limit, minLifetime.Seconds(), maxLifetime.Seconds())
	if err != nil {
		return nil, nil, err
	}

	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}

	defer rows.Close()

	return scanEmptyTasks(rows)
}

func scanEmptyTasks(rows pgx.Rows) (emptyTasks, filledTasks []*Task, err error) {
	emptyTasks = make([]*Task, 0)
	filledTasks = make([]*Task, 0)

	for rows.Next() {
		var (
			task     Task
			taskType string
		)

		err := rows.Scan(&task.WorkID, &task.VersionID, &task.WorkNumber, &task.Author, &task.RunContext, &taskType)
		if err != nil {
			return nil, nil, fmt.Errorf("scan task, %w", err)
		}

		switch taskType {
		case emptyType:
			emptyTasks = append(emptyTasks, &task)
		case filledType:
			filledTasks = append(filledTasks, &task)
		}
	}

	if rows.Err() != nil {
		return nil, nil, fmt.Errorf("rows.Err: %w", rows.Err())
	}

	return emptyTasks, filledTasks, nil
}

func (db *PGCon) GetTaskStepToRetry(ctx c.Context, taskID uuid.UUID) (*entity.Step, error) {
	ctx, span := trace.StartSpan(ctx, "task_step_to_retry")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT 
	    vs.id,
	    vs.work_id,
		w.work_number,
	    vs.step_type,
		vs.step_name, 
		vs.time, 
		vs.content, 
		COALESCE(vs.break_points, '{}') AS break_points, 
		vs.has_error,
		vs.status,
		w.author,
		vs.updated_at,
		w.run_context -> 'initial_application' -> 'is_test_application' as isTest,
		vs.is_paused
	FROM variable_storage vs 
	INNER JOIN works w ON vs.work_id = w.id
	WHERE 
	w.id = $1 AND
	vs.status IN ('new', 'ready')
	ORDER BY vs.time DESC
	LIMIT 1`

	var (
		s       entity.Step
		content string
	)

	err := db.Connection.QueryRow(ctx, q, taskID).Scan(
		&s.ID,
		&s.WorkID,
		&s.WorkNumber,
		&s.Type,
		&s.Name,
		&s.Time,
		&content,
		&s.BreakPoints,
		&s.HasError,
		&s.Status,
		&s.Initiator,
		&s.UpdatedAt,
		&s.IsTest,
		&s.IsPaused,
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

	return &s, nil
}
