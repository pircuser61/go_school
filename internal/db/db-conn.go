package db

import (
	"context"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/configs"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
)

type DBConn interface {
	ListPipelines(c context.Context) ([]model.Pipeline, error)
	AddPipeline(c context.Context) (*uuid.UUID, error)
	GetPipeline(c context.Context, id uuid.UUID) (*model.Pipeline, error)
	EditPipeline(c context.Context, id uuid.UUID, ) error
}

func DBConnect(db configs.Database) (DBConn, error) {
	switch db.Kind {
	case "postgres":
		return ConnectPostgres(db.Host, db.Port, db.DBName, db.User, db.Pass, db.MaxConnections, db.Timeout)
	case "mongo":
		return ConnectMongo(db.Host, db.Port, db.DBName, db.Timeout)
	}
	return nil, errors.New("unknown database")
}
