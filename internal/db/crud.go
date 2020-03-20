package db

import (
	"context"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/ctx"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
	"go.opencensus.io/trace"
)


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

func AddPipeline(c context.Context, pc *dbconn.PGConnection, pipeline []byte, name string) error {
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

func GetPipeline(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) (*model.Pipeline, error) {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return nil, nil
}

func EditPipeline(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(ctx.Context(60))
	if err != nil {
		return err
	}
	defer conn.Release()

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
