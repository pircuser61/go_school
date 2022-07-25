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
}

func (db *PGCon) CreateTask(c context.Context, dto *CreateTaskDTO) (*entity.EriusTask, error) {
	c, span := trace.StartSpan(c, "pg_create_task")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	tx, err := conn.Begin(c)
	if err != nil {
		return nil, err
	}

	var workNumber string

	if dto.WorkNumber == "" {
		workNumber, err = db.insertTask(c, tx, dto)
		if err != nil {
			return nil, err
		}
	} else {
		if err = db.setTaskChild(c, tx, dto.WorkNumber, dto.TaskID); err != nil {
			return nil, err
		}

		workNumber, err = db.insertTaskWithWorkNumber(c, tx, dto)
		if err != nil {
			return nil, err
		}
	}

	// nolint:gocritic
	// language=PostgreSQL
	const q = `UPDATE pipeliner.versions 
		SET last_run_id = $1 
		WHERE id = $2`

	_, err = tx.Exec(c, q, dto.TaskID, dto.VersionID)
	if err != nil {
		_ = tx.Rollback(c)

		return nil, err
	}

	if err = tx.Commit(c); err != nil {
		_ = tx.Rollback(c)

		return nil, err
	}

	return db.GetTask(c, workNumber)
}

func (db *PGCon) insertTaskWithWorkNumber(c context.Context, tx pgx.Tx, dto *CreateTaskDTO) (string, error) {
	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		INSERT INTO pipeliner.works(
			id, 
			version_id, 
			started_at, 
			status, 
			author, 
			debug, 
			parameters,
			work_number
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
		dto.WorkNumber,
	)

	var worksNumber string

	if err := row.Scan(&worksNumber); err != nil {
		_ = tx.Rollback(c)

		return "", err
	}

	return dto.WorkNumber, nil
}

func (db *PGCon) insertTask(c context.Context, tx pgx.Tx, dto *CreateTaskDTO) (workNumber string, err error) {
	// nolint:gocritic
	// language=PostgreSQL
	const query = `
		INSERT INTO pipeliner.works(
			id, 
			version_id, 
			started_at, 
			status, 
			author, 
			debug, 
			parameters
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
			$6, 
			$7
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
	)

	var worksNumber string

	if err = row.Scan(&worksNumber); err != nil {
		_ = tx.Rollback(c)

		return "", err
	}

	return dto.WorkNumber, nil
}
