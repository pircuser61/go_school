package dbconn

import (
	"strconv"

	"gitlab.services.mts.ru/erius/pipeliner/internal/ctx"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PGConnection struct {
	Pool *pgxpool.Pool
}

func ConnectPostgres(host, port, database, user, pass string, maxConn, timeout int) (*PGConnection, error) {
	maxConnections := strconv.Itoa(maxConn)
	connString := "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + database +
		"?sslmode=disable&pool_max_conns=" + maxConnections

	conn, err := pgxpool.Connect(ctx.Context(timeout), connString)
	if err != nil {
		return nil, err
	}

	pg := PGConnection{conn}

	return &pg, nil
}
