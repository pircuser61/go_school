package db

import (
	"context"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/ctx"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
	"go.opencensus.io/trace"
)

type PipelineStorageModel struct {
	ID       uuid.UUID
	Name     string
	Pipeline []byte
}

func ListPipelines(c context.Context, pc *dbconn.PGConnection) ([]model.Pipeline, error) {
	_, span := trace.StartSpan(c, "pg_list_pipelines")
	defer span.End()
	pipelines := make([]model.Pipeline, 0)
	conn, err := pc.Pool.Acquire(ctx.Context(60))
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	q := `SELECT id, name
	FROM public.pipelines;`
	rows, err := conn.Query(ctx.Context(60), q)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var pipeline model.Pipeline
		err = rows.Scan(&pipeline.ID, &pipeline.Name)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines, nil
}

func AddPipeline(c context.Context, pc *dbconn.PGConnection, name string, pipeline []byte) error {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return err
	}
	defer conn.Release()
	id := uuid.New()
	q := `
INSERT INTO public.pipelines(
	id, name, pipe)
	VALUES ($1, $2, $3);
`
	_, err = conn.Exec(c, q, id, name, pipeline)
	if err != nil {
		return nil
	}
	return nil
}

func GetPipeline(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) (*PipelineStorageModel, error) {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	q := `SELECT id, name, pipe
	FROM public.pipelines
	WHERE id = $1 LIMIT 1;`
	pipe := PipelineStorageModel{}
	rows, err := conn.Query(c, q, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		err := rows.Scan(&pipe.ID, &pipe.Name, &pipe.Pipeline)
		if err != nil {
			return nil, err
		}
	}
	return &pipe, nil
}

func EditPipeline(c context.Context, pc *dbconn.PGConnection, id uuid.UUID, pipeline []byte) error {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(ctx.Context(60))
	if err != nil {
		return err
	}
	defer conn.Release()

	q := `UPDATE public.pipelines
	SET pipe=$1
	WHERE id=$2;`
	_, err = conn.Exec(c, q, pipeline, id)
	if err != nil {
		return err
	}
	return nil
}

func WriteContext(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(ctx.Context(60))
	if err != nil {
		return err
	}
	defer conn.Release()

	return nil
}
