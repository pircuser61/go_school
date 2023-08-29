package db

import (
	c "context"
	"encoding/json"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (db *PGCon) deleteFinishedPipelineDeadlines(ctx c.Context, taskID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "delete_finished_pipeline_deadlines")
	defer span.End()

	q := `
		DELETE 
		FROM deadlines
		WHERE block_id IN (SELECT id FROM variable_storage WHERE work_id = $1)
	`
	_, err := db.Connection.Exec(ctx, q, taskID)
	return err
}

func (db *PGCon) deleteFinishedPipelineMembers(ctx context.Context, taskID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "delete_finished_pipeline_members")
	defer span.End()

	q := `
		DELETE 
		FROM members
		WHERE block_id IN (SELECT id FROM variable_storage WHERE work_id = $1)
	`
	_, err := db.Connection.Exec(ctx, q, taskID)
	return err
}

func (db *PGCon) UpdateTaskStatus(ctx c.Context, taskID uuid.UUID, status int, comment, author string) error {
	ctx, span := trace.StartSpan(ctx, "pg_change_task_status")
	defer span.End()

	var q string
	// nolint:gocritic
	// language=PostgreSQL
	switch status {
	case RunStatusCanceled, RunStatusFinished, RunStatusStopped:
		q = `UPDATE works 
		SET status = $1, finished_at = now(), status_comment = $3, status_author = $4
		WHERE id = $2`
		_, err := db.Connection.Exec(ctx, q, status, taskID, comment, author)
		if err != nil {
			return err
		}
	default:
		q = `UPDATE works 
		SET status = $1
		WHERE id = $2`
		_, err := db.Connection.Exec(ctx, q, status, taskID)
		if err != nil {
			return err
		}
	}

	switch status {
	case RunStatusFinished, RunStatusStopped, RunStatusError:
		if delErr := db.deleteFinishedPipelineDeadlines(ctx, taskID); delErr != nil {
			return delErr
		}
		if delErr := db.deleteFinishedPipelineMembers(ctx, taskID); delErr != nil {
			return delErr
		}
	}
	return nil
}

func (db *PGCon) UpdateTaskHumanStatus(ctx c.Context, taskID uuid.UUID, status string) (*entity.EriusTask, error) {
	ctx, span := trace.StartSpan(ctx, "update_task_human_status")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
		WITH is_parallel AS
		    (SELECT
				(SELECT COUNT(*)
					FROM variable_storage
			 		WHERE work_id = $2
			   			AND step_type = 'begin_parallel_task')
				>
				(SELECT COUNT(*)
					FROM variable_storage
					WHERE work_id = $2
						AND step_type = 'wait_for_all_inputs' 
						AND status = 'finished') AS result
		     )
		
		UPDATE works
		SET human_status = CASE
					   			WHEN $1 != 'cancel' AND $1 != 'revoke' AND (SELECT result FROM is_parallel) 
					   			    THEN 'processing'
					   			ELSE $1
						   END
		WHERE id = $2 RETURNING human_status, finished_at, work_number`

	row := db.Connection.QueryRow(ctx, q, status, taskID)

	et := entity.EriusTask{}

	err := row.Scan(
		&et.HumanStatus,
		&et.FinishedAt,
		&et.WorkNumber,
	)

	return &et, err
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

func (db *PGCon) UpdateTaskRate(ctx c.Context, req *UpdateTaskRate) (err error) {
	const q = `
		UPDATE works 
		SET 
			rate = $1,
			rate_comment = $2
		WHERE work_number = $3 AND author = $4`

	_, err = db.Connection.Exec(ctx, q, req.Rate, req.Comment, req.WorkNumber, req.ByLogin)

	return err
}

func (db *PGCon) SendTaskToArchive(ctx c.Context, taskID uuid.UUID) (err error) {
	ctx, span := trace.StartSpan(ctx, "send_task_to_archive")
	defer span.End()

	const q = `
		UPDATE works 
		SET archived = true
		WHERE id = $1`

	_, err = db.Connection.Exec(ctx, q, taskID)

	return err
}
