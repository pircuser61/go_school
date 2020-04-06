package dbconn

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/configs"
)

func DBConnect(db configs.Database) (*PGConnection, error) {
	return ConnectPostgres(db.Host, db.Port, db.DBName, db.User, db.Pass, db.MaxConnections, db.Timeout)
}
