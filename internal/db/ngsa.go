package db

import (
	"context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
)





func ActiveAlertNGSA(c context.Context, pc *dbconn.PGConnection, id uuid.UUID, reason string) error {

	return nil
}


func ClearAlertNGSA(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) error {

	return nil
}

func GetLingedAlertFromNGSA(c context.Context, pc *dbconn.PGConnection, name string) (uuid.UUID, error)  {

	return uuid.UUID{}, nil
}