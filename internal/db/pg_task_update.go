package db

import (
	c "context"
	"encoding/json"
	"fmt"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"github.com/google/uuid"
)

func (db *PGCon) UpdateTaskStatus(ctx c.Context, taskID uuid.UUID, status int) error {
	ctx, span := trace.StartSpan(ctx, "pg_change_task_status")
	defer span.End()

	var q string
	// nolint:gocritic
	// language=PostgreSQL
	if status == RunStatusFinished {
		q = `UPDATE works 
		SET status = $1, finished_at = now()
		WHERE id = $2`
	} else {
		q = `UPDATE works 
		SET status = $1
		WHERE id = $2`
	}

	_, err := db.Connection.Exec(ctx, q, status, taskID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) UpdateTaskHumanStatus(ctx c.Context, taskID uuid.UUID, status string) error {
	ctx, span := trace.StartSpan(ctx, "update_task_human_status")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `UPDATE works
		SET human_status = $1
		WHERE id = $2`

	_, err := db.Connection.Exec(ctx, q, status, taskID)
	return err
}

func (db *PGCon) setTaskChild(ctx c.Context, workNumber string, newTaskID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "set_task_child")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		UPDATE works
			SET child_id = $1
		WHERE child_id IS NULL AND work_number = $2`

	_, err := db.Connection.Exec(ctx, query, newTaskID, workNumber)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) UpdateTaskBlocksData(ctx c.Context, dto *UpdateTaskBlocksDataRequest) error {
	ctx, span := trace.StartSpan(ctx, "update_task_blocks_data")
	defer span.End()

	activeBlocks, err := json.Marshal(dto.ActiveBlocks)
	if err != nil {
		return errors.Wrap(err, "can`t marshal activeBlocks")
	}

	skippedBlocks, err := json.Marshal(dto.SkippedBlocks)
	if err != nil {
		return errors.Wrap(err, "can`t marshal skippedBlocks")
	}

	notifiedBlocks, err := json.Marshal(dto.NotifiedBlocks)
	if err != nil {
		return errors.Wrap(err, "can`t marshal notifiedBlocks")
	}

	prevUpdateStatusBlocks, err := json.Marshal(dto.PrevUpdateStatusBlocks)
	if err != nil {
		return errors.Wrap(err, "can`t marshal prevUpdateStatusBlocks")
	}

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		UPDATE works
			SET active_blocks = $2,
				skipped_blocks = $3,
				notified_blocks = $4,
				prev_update_status_blocks = $5
		WHERE id = $1`

	_, err = db.Connection.Exec(ctx, query, dto.Id, activeBlocks, skippedBlocks, notifiedBlocks, prevUpdateStatusBlocks)
	return err
}

func (db *PGCon) SetApplicationData(workNumber string, data *orderedmap.OrderedMap) error {
	q := `
		UPDATE variable_storage 
			SET content = JSONB_SET(content, '{State,servicedesk_application_0}', '%s')
		WHERE work_id = (SELECT id FROM works WHERE work_number = $1) AND
			step_type IN ('servicedesk_application', 'execution')`

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	q = fmt.Sprintf(q, string(bytes))
	_, err = db.Connection.Exec(c.Background(), q, workNumber)
	return err
}

func (db *PGCon) UpdateTaskRate(ctx c.Context, req *UpdateTaskRate) (err error) {
	const q = `
		update works 
		set 
			rate = $1,
			rate_comment = $2
		where work_number = $3 and author = $4`

	_, err = db.Connection.Exec(ctx, q, req.Rate, req.Comment, req.WorkNumber, req.ByLogin)

	return err
}
