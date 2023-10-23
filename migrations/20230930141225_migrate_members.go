package migrations

import (
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upMembers_, downMembers_)
}

func upMembers_(tx *sql.Tx) error {
	return nil
}

func downMembers_(tx *sql.Tx) error {
	return nil
}
