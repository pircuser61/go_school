package db

import "github.com/jackc/pgx"

func IsUniqueConstraintError(err error) bool {
	if pgerr, ok := err.(pgx.PgError); ok {
		if pgerr.Code == "23505" { // 23505 code - unique constraint violation
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
