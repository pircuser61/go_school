package db

import (
	c "context"

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
