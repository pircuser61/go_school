package db

import (
	"context"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type CreateTaskDTO struct {
	TaskID     uuid.UUID
	VersionID  uuid.UUID
	Author     string
	RealAuthor string
	WorkNumber string
	IsDebug    bool
	Params     []byte
	RunCtx     entity.TaskRunContext
}

func (db *PGCon) SetLastRunID(c context.Context, taskID, versionID uuid.UUID) error {
	// nolint:gocritic
	// language=PostgreSQL
	const q = `UPDATE versions 
		SET last_run_id = $1 
		WHERE id = $2`

	_, err := db.Connection.Exec(c, q, taskID, versionID)
	if err != nil {
		return err
	}

	return nil
}

func (db *PGCon) CreateTask(c context.Context, dto *CreateTaskDTO) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_create_task")
	defer span.End()

	var (
		workNumber string
		err        error
	)

	if dto.WorkNumber == "" {
		workNumber, err = db.insertTask(c, dto)
		if err != nil {
			return nil, err
		}
	} else {
		workNumber, err = db.createTaskWithWorkNumber(c, dto)
		if err != nil {
			return nil, err
		}
	}

	return db.GetTask(c, []string{dto.Author}, []string{dto.Author}, dto.Author, workNumber)
}

func (db *PGCon) createTaskWithWorkNumber(ctx context.Context, dto *CreateTaskDTO) (string, error) {
	err := db.setTaskChild(ctx, dto.WorkNumber, dto.TaskID)
	if err != nil {
		return "", err
	}

	workNumber, err := db.insertTaskWithWorkNumber(ctx, dto)
	if err != nil {
		return "", err
	}

	err = db.FinishTaskBlocks(ctx, dto.TaskID, nil, true)
	if err != nil {
		return "", err
	}

	return workNumber, nil
}

func (db *PGCon) insertTaskWithWorkNumber(c context.Context, dto *CreateTaskDTO) (string, error) {
	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		INSERT INTO works(
			id, 
			version_id, 
			started_at, 
			status, 
			author, 
			debug, 
			parameters,
			work_number,
			run_context
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
			$6, 
			$7,
			$8,
			$9
		)
	RETURNING work_number
`

	row := db.Connection.QueryRow(
		c,
		query,
		dto.TaskID,
		dto.VersionID,
		time.Now(),
		RunStatusCreated,
		dto.Author,
		dto.IsDebug,
		dto.Params,
		dto.WorkNumber,
		dto.RunCtx,
	)

	var worksNumber string

	if err := row.Scan(&worksNumber); err != nil {
		return "", err
	}

	return worksNumber, nil
}

func (db *PGCon) insertTask(c context.Context, dto *CreateTaskDTO) (workNumber string, err error) {
	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		INSERT INTO works(
			id, 
			version_id, 
			started_at, 
			status, 
			author, 
			debug, 
			parameters,
			run_context,
			real_author
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
			$6, 
			$7,
			$8,
		    $9
		)
	RETURNING work_number
`

	row := db.Connection.QueryRow(
		c,
		query,
		dto.TaskID,
		dto.VersionID,
		time.Now(),
		RunStatusCreated,
		dto.Author,
		dto.IsDebug,
		dto.Params,
		dto.RunCtx,
		dto.RealAuthor,
	)

	var worksNumber string

	if scanErr := row.Scan(&worksNumber); scanErr != nil {
		return "", scanErr
	}

	return worksNumber, nil
}
