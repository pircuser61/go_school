package db

import (
	"context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"time"
)

const (
	Active = "ACTIVE"
	Clear  = "CLEAR"
)

func ActiveAlertNGSA(c context.Context, pc *dbconn.PGConnection, id uuid.UUID, severn int, source, eventType,
	cause, addInf, addTxt, moId, specProb, notID, usertext, moInstance, moClass string) error {
	state := Active
	t := time.Now()
	q := `INSERT INTO pipeliner.ngsa_alert(
		id, state, "perceivedSeverity", "eventSource", "eventTime", "eventType", "probableCause", 
		"additionalInformation", "additionalText", "moIdentifier", "specificProblem", "notificationIdentifier", 
		"userText", managedobjectinstance, managedobjectclass)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);`
	_, err := pc.Pool.Exec(c, q, id, state, severn, source, t, eventType, cause, addInf, addTxt, moId, specProb, notID,
		usertext, moInstance, moClass)
	return err
}

func ClearAlertNGSA(c context.Context, pc *dbconn.PGConnection, id uuid.UUID) error {

	return nil
}

func GetLingedAlertFromNGSA(c context.Context, pc *dbconn.PGConnection, name string) (uuid.UUID, error) {

	return uuid.UUID{}, nil
}
