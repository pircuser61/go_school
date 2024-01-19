package migrations

import (
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upMembers_, downMembers_)
}

//nolint:revive //метод upMembers уже существует
func upMembers_(_ *sql.Tx) error {
	return nil
}

//nolint:revive //метод downMembers уже существует
func downMembers_(_ *sql.Tx) error {
	return nil
}
