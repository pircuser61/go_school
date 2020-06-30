package db

import (
	"context"
	"time"

	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
)

func ActiveAlertNGSA(c context.Context, pc *dbconn.PGConnection, sever int,
	state, source, eventType, cause, addInf, addTxt, moId, specProb, notID, usertext, moi, moc string) error {
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
	VALUES ($1, $2, $3, $4, $5, $6, $7,$8, $9, $10, $11, $12, $13, $14);`
	_, err := pc.Pool.Exec(c, q, state, sever, source, t, eventType, cause,
		addInf, addTxt, moId, specProb, notID, usertext, moi, moc)
	return err
}

func ClearAlertNGSA(c context.Context, pc *dbconn.PGConnection, name string) error {
	t := time.Now()
	q := `UPDATE pipeliner.alarm_for_ngsa SET
	state = 'CLEAR', cleartime = $1
	WHERE "notificationIdentifier" = $2
`
	_, err := pc.Pool.Exec(c, q, t, name)
	return err
}
