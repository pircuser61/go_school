package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upMembers__, downMembers__)
}

//nolint:revive //функция upMembers уже сущестует
func upMembers__(_ *sql.Tx) error {
	return nil
}

//nolint:revive //функция downMembers уже сущестует
func downMembers__(_ *sql.Tx) error {
	return nil
}
