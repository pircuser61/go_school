package db

import (
	"context"
	"time"

	"go.opencensus.io/trace"

	"github.com/jackc/pgx/v4"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type CreateTaskDTO struct {
	TaskID     uuid.UUID
	VersionID  uuid.UUID
	Author     string
	WorkNumber string
	IsDebug    bool
	Params     []byte
	RunCtx     entity.TaskRunContext
}

func (db *PGCon) CreateTask(c context.Context, tx pgx.Tx, dto *CreateTaskDTO) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_create_task")
	defer span.End()

	var workNumber string
	var err error

	if dto.WorkNumber == "" {
		workNumber, err = db.insertTask(c, tx, dto)
		if err != nil {
			return nil, err
		}
	} else {
		err = db.setTaskChild(c, tx, dto.WorkNumber, dto.TaskID)
		if err != nil {
			return nil, err
		}

		workNumber, err = db.insertTaskWithWorkNumber(c, tx, dto)
		if err != nil {
			return nil, err
		}
	}

	// nolint:gocritic
	// language=PostgreSQL
	const q = `UPDATE versions 
		SET last_run_id = $1 
		WHERE id = $2`

	_, err = tx.Exec(c, q, dto.TaskID, dto.VersionID)
	if err != nil {
		_ = tx.Rollback(c)

		return nil, err
	}

	return db.GetTask(c, workNumber)
}

func (db *PGCon) insertTaskWithWorkNumber(c context.Context, tx pgx.Tx, dto *CreateTaskDTO) (string, error) {
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

	row := tx.QueryRow(
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
		_ = tx.Rollback(c)

		return "", err
	}

	return worksNumber, nil
}

func (db *PGCon) insertTask(c context.Context, tx pgx.Tx, dto *CreateTaskDTO) (workNumber string, err error) {
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
			$8
		)
	RETURNING work_number
`

	row := tx.QueryRow(
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
	)

	var worksNumber string

	if err = row.Scan(&worksNumber); err != nil {
		_ = tx.Rollback(c)

		return "", err
	}

	return worksNumber, nil
}
