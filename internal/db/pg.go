package db

import (
	"context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
	"go.opencensus.io/trace"
	"strconv"

	"gitlab.services.mts.ru/erius/pipeliner/internal/ctx"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PGConnection struct {
	Pool *pgxpool.Pool
}

func ConnectPostgres(host, port, database, user, pass string, maxConn, timeout int) (*PGConnection, error) {
	maxConnections := strconv.Itoa(maxConn)
	connString := "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + database +
		"?sslmode=disable&pool_max_conns=" + maxConnections
	conn, err := pgxpool.Connect(ctx.Context(timeout), connString)
	if err != nil {
		return nil, err
	}
	pg := PGConnection{conn}
	return &pg, nil
}

func (pc *PGConnection) ListPipelines(c context.Context) ([]model.Pipeline, error) {
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

func (pc *PGConnection) AddPipeline(c context.Context) (*uuid.UUID, error) {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return nil, nil
}


func (pc *PGConnection) GetPipeline(c context.Context, id uuid.UUID) (*model.Pipeline, error) {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(c)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return nil, nil
}

func (pc *PGConnection) EditPipeline(c context.Context, id uuid.UUID) error {
	c, span := trace.StartSpan(c, "pg_add_pipeline")
	defer span.End()
	conn, err := pc.Pool.Acquire(ctx.Context(60))
	if err != nil {
		return err
	}
	defer conn.Release()

	return nil
}
