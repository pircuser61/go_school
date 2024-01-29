package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

//nolint:gochecknoinits //необходимо для гуся
func init() {
	goose.AddMigration(upMembers__, downMembers__)
}

//nolint:revive,stylecheck //функция upMembers уже сущестует
func upMembers__(_ *sql.Tx) error {
	return nil
}

//nolint:revive,stylecheck //функция downMembers уже сущестует
func downMembers__(_ *sql.Tx) error {
	return nil
}
