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

func ActiveAlertNGSA(c context.Context, pc *dbconn.PGConnection, severn int, source, eventType,
	cause, addInf, addTxt, moId, specProb, notID, usertext, moInstance, moClass string) error {
	t := time.Now()
	q := `INSERT INTO pipeliner.alarm_for_ngsa(
		state, 
		"perceivedSeverity",
		"eventSource", 
		"eventTime",
		"eventType", 
		"probableCause", 
		"additionInformation", 
		"additionalText", 
		"moIdentifier", 
		"specificProblem", 
		"notificationIdentifier",
		"userText", 
		managedobjectinstance,
		managedobjectclass)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);`
	_, err := pc.Pool.Exec(c, q, Active, severn, source, t, eventType, cause, addInf, addTxt, moId,
		specProb, notID, usertext, moInstance, moClass)
	return err
}

func ClearAlertNGSA(c context.Context, pc *dbconn.PGConnection, name string) error {
	t := time.Now()
	q := `UPDATE pipeliner.alarm_for_ngsa set state = $1, cleartime = $2 where "notificationIdentifier" = $3`
	_, err := pc.Pool.Exec(c, q, Clear, t, name)
	return err
}

func GetLingedAlertFromNGSA(c context.Context, pc *dbconn.PGConnection, notificaton string) (uuid.UUID, error) {
	//q : = `select id from pipeliner `
	return uuid.UUID{}, nil
}
