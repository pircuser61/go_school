package db

import (
	"context"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/ctx"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoConnection struct {
	db *mongo.Database
}

func ConnectMongo(host, port, database string, timeout int) (*MongoConnection, error) {
	connString := "mongodb://" + host + ":" + port
	client, err := mongo.NewClient(options.Client().ApplyURI(connString))
	if err != nil {
		return nil, errors.Errorf("can't create mongodb client: %w", err)
	}
	err = client.Connect(ctx.Context(timeout))
	if err != nil {
		return nil, errors.Errorf("can't connect to mongodb: %w", err)
	}
	db := client.Database(database)
	mc := MongoConnection{db}
	return &mc, nil
}

func (mc *MongoConnection) ListPipelines(c context.Context) ([]model.Pipeline, error) {

	return nil, nil
}
func (pc *MongoConnection) AddPipeline(c context.Context) (*uuid.UUID, error) {

	return nil, nil
}


func (pc *MongoConnection) GetPipeline(c context.Context, id uuid.UUID) (*model.Pipeline, error) {

	return nil, nil
}

func (pc *MongoConnection) EditPipeline(c context.Context, id uuid.UUID) error {

	return nil
}
