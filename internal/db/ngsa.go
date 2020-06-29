package db

import (
	"context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
)

const (
	Active = "ACTIVE"
	Clear  = "CLEAR"
)

func ActiveAlertNGSA(c context.Context, pc *dbconn.PGConnection) error {
	//t := time.Now()
	q := ``
	_, err := pc.Pool.Exec(c, q)
	return err
}

func ClearAlertNGSA(c context.Context, pc *dbconn.PGConnection, name string) error {
	//t := time.Now()
	q := ``
	_, err := pc.Pool.Exec(c, q)
	return err
}

func GetLingedAlertFromNGSA(c context.Context, pc *dbconn.PGConnection, notificaton string) (uuid.UUID, error) {
	//q : = `select id from pipeliner `
	return uuid.UUID{}, nil
}
