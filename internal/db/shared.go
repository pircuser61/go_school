package db

import (
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
)

func IsUniqueConstraintError(err error) bool {
	if pgerr, ok := err.(*pgconn.PgError); ok {
		if pgerr.Code == pgerrcode.UniqueViolation { // 23505 code - unique constraint violation
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
