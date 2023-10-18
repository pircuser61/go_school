package migrations

import (
	"database/sql"
	
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upMembers__, downMembers__)
}

func upMembers__(tx *sql.Tx) error {

	return nil
}

func downMembers__(tx *sql.Tx) error {
	return nil
}
