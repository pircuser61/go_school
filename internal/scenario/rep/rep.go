package rep

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"go.opencensus.io/trace"
	"time"
)

var (
	StatusDraft int = 1
)

var (
	errCantFindPipelineVersion = errors.New("can't find pipeline version")
	errCantFindTag             = errors.New("can't find tag")
)

type ScenarioRepository struct {
	Pool *pgxpool.Pool
}

func NewScenarioRepository(conn db.PGConnection) *ScenarioRepository {
	return &ScenarioRepository{
		Pool: conn.Pool,
	}
}

func (db *ScenarioRepository) CreatePipeline(c context.Context, p *entity.EriusScenarioV2, author string, pipelineData []byte) error {
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

func (db *ScenarioRepository) GetPipelineVersion(c context.Context, id string) (*entity.EriusScenarioV2, error) {
	c, span := trace.StartSpan(c, "pg_get_pipeline_version")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}

	defer conn.Release()

	p := entity.EriusScenarioV2{}

	// nolint:gocritic
	// language=PostgreSQL
	qVersion := `
	SELECT 
		pv.id, 
		pv.status, 
		pv.pipeline_id, 
		pv.created_at, 
		pv.content, 
		pv.comment_rejected, 
		pv.comment, 
		pv.author, 
		pph.date
	FROM pipeliner.versions pv
    LEFT JOIN pipeliner.pipeline_history pph ON pph.version_id = pv.id
	WHERE pv.id = $1
	ORDER BY pph.date DESC 
	LIMIT 1`

	rows, err := conn.Query(c, qVersion, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var (
			vID, pID uuid.UUID
			s        int
			c        string
			cr       string
			cm       string
			d        *time.Time
			ca       *time.Time
			a        string
		)

		err := rows.Scan(&vID, &s, &pID, &ca, &c, &cr, &cm, &a, &d)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(c), &p)
		if err != nil {
			return nil, err
		}

		p.VersionID = vID
		p.ID = pID
		p.Status = s
		p.CommentRejected = cr
		p.Comment = cm
		p.ApprovedAt = d
		p.CreatedAt = ca
		p.Author = a

		return &p, nil
	}

	return nil, fmt.Errorf("%w: with id: %v", errCantFindPipelineVersion, id)
}

func (db *ScenarioRepository) IsScenarioNameExist(c context.Context, name string) (bool, error) {
	c, span := trace.StartSpan(c, "pg_pipeline_name_creatable")
	defer span.End()

	conn, err := db.Pool.Acquire(c)
	if err != nil {
		return false, err
	}

	defer conn.Release()

	// nolint:gocritic
	// language=PostgreSQL
	q := `
	SELECT count(name) AS count
	FROM pipeliner.pipelines
	WHERE name = $1`

	row := conn.QueryRow(c, q, name)

	count := 0

	err = row.Scan(&count)
	if err != nil {
		return false, err
	}

	if count != 0 {
		return true, nil
	}

	return false, nil
}
