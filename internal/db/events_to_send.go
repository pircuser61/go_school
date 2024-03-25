package db

import (
	c "context"
	"encoding/json"

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

func (db *PGCon) UpdateEventToSend(ctx c.Context, eventID string) (err error) {
	ctx, span := trace.StartSpan(ctx, "update_event_to_send")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		UPDATE events_to_send 
			SET sent_at = now()
		WHERE id = $1`

	_, err = db.Connection.Exec(ctx, q, eventID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) GetEventsToSend(ctx c.Context) ([]e.ToSendKafkaEvent, error) {
	ctx, span := trace.StartSpan(ctx, "get_events_to_send")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		SELECT id, message 
			FROM events_to_send
				WHERE sent_at IS NULL
			ORDER BY created_at
		LIMIT 100`

	rows, err := db.Connection.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]e.ToSendKafkaEvent, 0)

	for rows.Next() {
		var eventID, message string

		item := e.NodeKafkaEvent{}

		if scanErr := rows.Scan(&eventID, &message); scanErr != nil {
			return nil, scanErr
		}

		err = json.Unmarshal([]byte(message), &item)
		if err != nil {
			return nil, err
		}

		items = append(items, e.ToSendKafkaEvent{EventID: eventID, Event: item})
	}

	return items, nil
}
