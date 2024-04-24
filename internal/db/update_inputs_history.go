package db

import (
	c "context"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"go.opencensus.io/trace"
)

func (db *PGCon) CreateUpdatesInputsHistory(ctx c.Context, in *e.CreateUpdatesInputsHistory) (err error) {
	ctx, span := trace.StartSpan(ctx, "create_updates_inputs_history")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const q = `
		INSERT INTO update_inputs_history (
			work_id, 
			event_id, 
			step_name, 
			author, 
			content, 
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

	_, err = db.Connection.Exec(ctx, q, in.WorkID, in.EventID, in.StepName, in.Author, in.Inputs)
	if err != nil {
		return err
	}

	return nil
}
