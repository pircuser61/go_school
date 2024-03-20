package db

import (
	c "context"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (db *PGCon) CreateEventToSend(ctx c.Context, dto *e.CreateEventToSend) (eventID string, err error) {
	ctx, span := trace.StartSpan(ctx, "create_event_to_send")
	defer span.End()

	eventID = uuid.New().String()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		INSERT INTO events_to_send (
			id, 
			work_id, 
			message, 
			created_at
		)
		VALUES (
			$1, 
			$2, 
			$3, 
		    now()
		)`

	_, err = db.Connection.Exec(ctx, q, eventID, dto.WorkID, dto.Message)
	if err != nil {
		return eventID, err
	}

	return eventID, nil
}
