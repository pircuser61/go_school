package db

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type UpdateEmptyTaskDTO struct {
	WorkID uuid.UUID

	VersionID  uuid.UUID
	RealAuthor string
	Parameters []byte
	Debug      bool
	RunContext entity.TaskRunContext
}

//nolint:gocritic //ну че нам памяти что-ли жалко
func NewUpdateEmptyTaskDTO(
	workID, versionID uuid.UUID,
	realAuthor string,
	parameters []byte,
	runContext entity.TaskRunContext,
) UpdateEmptyTaskDTO {
	return UpdateEmptyTaskDTO{
		WorkID:     workID,
		VersionID:  versionID,
		RunContext: runContext,
		RealAuthor: realAuthor,
		Parameters: parameters,
		Debug:      false,
	}
}

func (db *PGCon) FillEmptyTask(ctx c.Context, updateTask *UpdateEmptyTaskDTO) error {
	ctx, span := trace.StartSpan(ctx, "update_task")
	defer span.End()

	const versionSLAQuery = `
		SELECT id FROM version_sla
			WHERE version_id = $1
		ORDER BY created_at DESC LIMIT 1
	`

	var slaID uuid.UUID

	err := db.Connection.QueryRow(ctx, versionSLAQuery, updateTask.VersionID).Scan(&slaID)
	if err != nil {
		return fmt.Errorf("failed get version_sla_id for version_id = %s, %w", updateTask.VersionID, err)
	}

	const updateQuery = `
			UPDATE works 
			SET 
			version_id = $2,
			run_context = $3,
			version_sla_id = $4,
			real_author = $5,
			parameters = $6,
			debug = $7
			
			WHERE id = $1 
	`

	_, err = db.Connection.Exec(
		ctx,
		updateQuery,
		updateTask.WorkID,
		updateTask.VersionID,
		updateTask.RunContext,
		slaID,
		updateTask.RealAuthor,
		updateTask.Parameters,
		updateTask.Debug,
	)
	if err != nil {
		return fmt.Errorf("failed update task by work_id %s, %w", updateTask.WorkID, err)
	}

	return nil
}

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

func (db *PGCon) deleteFinishedPipelineMembers(ctx c.Context, taskID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "delete_finished_pipeline_members")
	defer span.End()

	q := `
		DELETE 
		FROM members
		WHERE block_id IN (SELECT id FROM variable_storage WHERE work_id = $1)
		AND is_acted = false AND execution_group_member = false
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
	case RunStatusCanceled, RunStatusFinished, RunStatusStopped, RunStatusError:
		if delErr := db.deleteFinishedPipelineDeadlines(ctx, taskID); delErr != nil {
			return delErr
		}

		if delErr := db.deleteFinishedPipelineMembers(ctx, taskID); delErr != nil {
			return delErr
		}
	}

	return nil
}

func (db *PGCon) UpdateTaskHumanStatus(ctx c.Context, taskID uuid.UUID, status, comment string) (*entity.EriusTask, error) {
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
		    					WHEN $1 IN ('cancel', 'revoke', 'rejected', 'approvement-reject', 'executor-reject')
		    						THEN $1
		    					WHEN (SELECT result FROM is_parallel) 
									THEN 'processing'
								ELSE $1
						   END,
		    human_status_comment = $3
		WHERE id = $2 RETURNING human_status, finished_at, work_number`

	row := db.Connection.QueryRow(ctx, q, status, taskID, comment)

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

	_, err = db.Connection.Exec(ctx, query, dto.ID, activeBlocks, skippedBlocks, notifiedBlocks, prevUpdateStatusBlocks)

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

func (db *PGCon) UpdateBlockStateInOthers(ctx c.Context, blockName, taskID string, blockState []byte) error {
	ctx, span := trace.StartSpan(ctx, "update_block_state_in_others")
	defer span.End()

	const q = `
		UPDATE variable_storage 
		SET content = jsonb_set(content, array['State', $1]::varchar[], $2::jsonb, false)
		WHERE work_id = $3
			AND time >= (
			    SELECT max(time)
			    FROM variable_storage
			    WHERE step_name = $1
			    	AND work_id = $3
			)`

	_, err := db.Connection.Exec(ctx, q, blockName, blockState, taskID)

	return err
}

func (db *PGCon) UpdateBlockVariablesInOthers(ctx c.Context, taskID string, values map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "update_block_variables_in_others")
	defer span.End()

	for varName := range values {
		const q = `
		UPDATE variable_storage 
		SET content = jsonb_set(content, array['Values', $1]::varchar[], $2::jsonb, false)
		WHERE work_id = $3`

		_, err := db.Connection.Exec(ctx, q, varName, values[varName], taskID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *PGCon) SetExecDeadline(ctx c.Context, taskID string, deadline time.Time) error {
	ctx, span := trace.StartSpan(ctx, "set_block_variables_in_others")
	defer span.End()

	const q = `UPDATE works SET exec_deadline = $1 WHERE id = $2`

	_, err := db.Connection.Exec(ctx, q, deadline, taskID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) SetTaskPaused(ctx c.Context, workID string, pause bool) error {
	ctx, span := trace.StartSpan(ctx, "set_task_paused")
	defer span.End()

	const q = `UPDATE works SET is_paused = $1, status = $2 WHERE id = $3`

	status := 4
	if !pause {
		status = 1
	}

	_, err := db.Connection.Exec(ctx, q, pause, status, workID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) SetTaskBlocksPaused(ctx c.Context, workID string, steps []string, isPaused bool) error {
	ctx, span := trace.StartSpan(ctx, "set_task_blocks_paused")
	defer span.End()

	if len(steps) == 0 {
		q := `UPDATE variable_storage SET is_paused = $1 
          	WHERE work_id = $2 AND status IN('running', 'idle', 'created', 'ready')`

		_, err := db.Connection.Exec(ctx, q, isPaused, workID)
		if err != nil {
			return err
		}

		return nil
	}

	q := `
		UPDATE variable_storage SET is_paused = $1 
		WHERE work_id = $2 AND
			  status IN('running', 'idle', 'created', 'ready') AND
			  id = ANY ($3)`

	stepsIn := pq.StringArray(steps)

	_, err := db.Connection.Exec(ctx, q, isPaused, workID, stepsIn)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) UnpauseTaskBlock(ctx c.Context, workID, stepID uuid.UUID) (err error) {
	ctx, span := trace.StartSpan(ctx, "unpause_task_block")
	defer span.End()

	const q = `UPDATE variable_storage SET is_paused = false WHERE id = $1 AND work_id = $2`

	_, err = db.Connection.Exec(ctx, q, stepID, workID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) TryUnpauseTask(ctx c.Context, workID uuid.UUID) (err error) {
	ctx, span := trace.StartSpan(ctx, "try_unpause_task")
	defer span.End()

	var i int

	const q = `
		SELECT count(id)
		FROM variable_storage
		WHERE work_id = $1 AND is_paused = true AND 
			status IN('running','idle','created','ready')`

	err = db.Connection.QueryRow(ctx, q, workID).Scan(&i)
	if err != nil {
		return err
	}

	if i != 0 {
		return nil
	}

	err = db.SetTaskPaused(ctx, workID.String(), false)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) SkipBlocksAfterRestarted(ctx c.Context, workID uuid.UUID, startTime time.Time, blocks []string) (err error) {
	ctx, span := trace.StartSpan(ctx, "skip_blocks_after_restarted")
	defer span.End()

	blocksDB := pq.StringArray(blocks)

	const q = `UPDATE variable_storage SET status = 'skipped' 
                WHERE work_id = $1 AND step_name = ANY ($2) AND time > $3`

	_, err = db.Connection.Exec(ctx, q, workID, blocksDB, startTime)
	if err != nil {
		return err
	}

	return nil
}
