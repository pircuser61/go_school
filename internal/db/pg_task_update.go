package db

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"

	"github.com/jackc/pgx/v4"

	"go.opencensus.io/trace"

	"github.com/google/uuid"
)

func (db *PGCon) ChangeTaskStatus(c context.Context,
	taskID uuid.UUID, status int) error {
	c, span := trace.StartSpan(c, "pg_change_task_status")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	var q string
	// nolint:gocritic
	// language=PostgreSQL
	if status == RunStatusFinished {
		q = `UPDATE pipeliner.works 
		SET status = $1, finished_at = now()
		WHERE id = $2`
	} else {
		q = `UPDATE pipeliner.works 
		SET status = $1
		WHERE id = $2`
	}

	_, err = conn.Exec(c, q, status, taskID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) UpdateTaskHumanStatus(c context.Context, taskID uuid.UUID, status string) error {
	c, span := trace.StartSpan(c, "update_task_human_status")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}
	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `UPDATE pipeliner.works
		SET human_status = $1
		WHERE id = $2`

	_, err = conn.Exec(c, q, status, taskID)
	return err
}

func (db *PGCon) setTaskChild(c context.Context, tx pgx.Tx, workNumber string, newTaskID uuid.UUID) error {
	c, span := trace.StartSpan(c, "set_task_child")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}
	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		UPDATE pipeliner.works
			SET child_id = $1
		WHERE child_id IS NULL AND work_number = $2`

	_, err = tx.Exec(c, query, newTaskID, workNumber)
	if err != nil {
		_ = tx.Rollback(c)
	}

	return err
}

func (db *PGCon) UpdateTaskBlocksData(c context.Context, dto *UpdateTaskBlocksDataRequest) error {
	c, span := trace.StartSpan(c, "update_task_blocks_data")
	defer span.End()

	activeBlocks, err := json.Marshal(dto.ActiveBlocks)
	if err != nil {
		return errors.Wrap(err,"can`t marshal activeBlocks")
	}

	skippedBlocks, err := json.Marshal(dto.SkippedBlocks)
	if err != nil {
		return errors.Wrap(err,"can`t marshal skippedBlocks")
	}

	notifiedBlocks, err := json.Marshal(dto.NotifiedBlocks)
	if err != nil {
		return errors.Wrap(err,"can`t marshal notifiedBlocks")
	}

	prevUpdateStatusBlocks, err := json.Marshal(dto.PrevUpdateStatusBlocks)
	if err != nil {
		return errors.Wrap(err,"can`t marshal prevUpdateStatusBlocks")
	}

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}
	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		UPDATE pipeliner.works
			SET active_blocks = $2,
				skipped_blocks = $3,
				notified_blocks = $4,
				prev_update_status_blocks = $5
		WHERE id = $1`

	_, err = conn.Exec(c, query, dto.Id, activeBlocks, skippedBlocks, notifiedBlocks, prevUpdateStatusBlocks)
	return err
}