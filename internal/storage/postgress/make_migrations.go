package storage

import (
	"github.com/jackc/pgx/v5/stdlib"
	goose "github.com/pressly/goose/v3"

	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

func MakeMigrations(pool *pgxpool.Pool) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	db := stdlib.OpenDBFromPool(pool)
	if err := goose.Up(db, "./../../migrations/"); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	return nil
}
