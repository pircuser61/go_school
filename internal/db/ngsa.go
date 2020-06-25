package db

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
)

const (
	Active = "ACTIVE"
	Clear = "CLEAR"
)

func ActiveAlertNGSA(c context.Context, pc *dbconn.PGConnection) error {
	q := `INSERT INTO pipeliner.ngsa_alert(
		id, state, "perceivedSeverity", "eventSource", "eventTime", "eventType", "probableCause", 
		"additionalInformation", "additionalText", "moIdentifier", "specificProblem", "notificationIdentifier", 
		"userText", managedobjectinstance, managedobjectclass, cleartime)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);`
	fmt.Println(q)
	return nil
}


func ClearAlertNGSA(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) error {

	return nil
}

func GetLingedAlertFromNGSA(c context.Context, pc *dbconn.PGConnection, name string) (uuid.UUID, error)  {

	return uuid.UUID{}, nil
}