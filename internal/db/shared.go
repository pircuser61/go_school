package db

import (
	"github.com/jackc/pgconn"

	"github.com/jackc/pgerrcode"
)

func IsUniqueConstraintError(err error) bool {
	if pgerr, ok := err.(*pgconn.PgError); ok {
		return pgerr.Code == pgerrcode.UniqueViolation
	}

	return false
}
