package storage

import (
	"context"
	"log/slog"
	"time"

	//config "github.com/pircuser61/go_school/config"
	models "github.com/pircuser61/go_school/internal/models"
	storage "github.com/pircuser61/go_school/internal/storage"
)

type PostgresStore struct {
	l       *slog.Logger
	Timeout time.Duration
}

func New(ctx context.Context, logger *slog.Logger) (storage.MateralStore, error) {
	store := PostgresStore{l: logger}
	return store, nil
}

func (c PostgresStore) Close() {
	c.l.Info("Соедиение с БД закрыто ")
}

func (c PostgresStore) Materials(ctx context.Context, filter models.MaterialListFilter) ([]*models.Material, error) {
	result := []*models.Material{{Name: "TEST"}}
	time.Sleep(time.Second * 10)
	return result, nil
}
