package db

import (
	c "context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"github.com/google/uuid"

	"go.opencensus.io/trace"
)

func (db *PGCon) CreateTaskEvent(ctx c.Context, dto *e.CreateTaskEvent) (eventID string, err error) {
	ctx, span := trace.StartSpan(ctx, "create_task_event")
	defer span.End()

	eventID = uuid.New().String()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		INSERT INTO task_events (
			id, 
			work_id, 
			author, 
			event_type, 
			params, 
			created_at
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4,
			$5,
		    now()
		)`

	_, err = db.Connection.Exec(ctx, q, eventID, dto.WorkID, dto.Author, dto.EventType, dto.Params)
	if err != nil {
		return eventID, err
	}

	return eventID, nil
}

func (db *PGCon) GetTaskEvents(ctx c.Context, workID string) ([]e.TaskEvent, error) {
	ctx, span := trace.StartSpan(ctx, "get_task_events")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		SELECT 
			id, 
			work_id, 
			author, 
			event_type, 
			params, 
			created_at
		FROM task_events
		WHERE work_id = $1
		ORDER BY created_at`

	rows, err := db.Connection.Query(ctx, q, workID)
	if err != nil {
		return nil, err
	}

	events := make([]e.TaskEvent, 0)

	defer rows.Close()

	for rows.Next() {
		event := e.TaskEvent{}

		scanErr := rows.Scan(
			&event.ID,
			&event.WorkID,
			&event.Author,
			&event.EventType,
			&event.Params,
			&event.CreatedAt,
		)
		if scanErr != nil {
			return nil, scanErr
		}

		events = append(events, event)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return events, nil
}

