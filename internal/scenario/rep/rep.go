package rep

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"go.opencensus.io/trace"
	"time"
)

type ScenarioRepository struct {
	Pool *pgxpool.Pool
}

func NewScenarioRepository() *ScenarioRepository {
	return &ScenarioRepository{}
}

func (db *ScenarioRepository) CreatePipeline(c context.Context,
	p *entity.EriusScenarioV2, author string, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_create_pipeline")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return err
	}

	defer conn.Release()

	tx, err := conn.Begin(c)
	if err != nil {
		return err
	}

	createdAt := time.Now()

	// nolint:gocritic
	// language=PostgreSQL
	qNewPipeline := `
	INSERT INTO pipeliner.pipelines (
		id, 
		name, 
		created_at, 
		author
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4
	)`

	_, err = tx.Exec(c, qNewPipeline, p.ID, p.Name, createdAt, author)
	if err != nil {
		return err
	}

	err = tx.Commit(c)
	if err != nil {
		_ = tx.Rollback(c)

		return err
	}

	return db.CreateVersion(c, p, author, pipelineData)
}

func (db *ScenarioRepository) CreateVersion(c context.Context,
	p *entity.EriusScenarioV2, author string, pipelineData []byte) error {
	c, span := trace.StartSpan(c, "pg_create_version")
	defer span.End()

	// nolint:gocritic
	// language=PostgreSQL
	qNewVersion := `
	INSERT INTO pipeliner.versions (
		id, 
		status, 
		pipeline_id, 
		created_at, 
		content, 
		author, 
		comment
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4, 
		$5, 
		$6, 
		$7
	)`

	createdAt := time.Now()

	_, err := db.Pool.Exec(c, qNewVersion, p.VersionID, StatusDraft, p.ID, createdAt, pipelineData, author, p.Comment)
	if err != nil {
		return err
	}

	return nil
}
