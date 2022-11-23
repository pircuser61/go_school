package db

import (
	"context"
	"time"
)

func (db *PGCon) ActiveAlertNGSA(c context.Context, sever int,
	state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error {
	t := time.Now()
	// nolint:gocritic
	// language=PostgreSQL
	q := `
	INSERT INTO alarm_for_ngsa (
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
		managedobjectclass
	)
	VALUES (
		$1, 
		$2, 
		$3, 
		$4, 
		$5, 
		$6, 
		$7, 
		$8, 
		$9, 
		$10, 
		$11, 
		$12, 
		$13, 
		$14
	)`
	_, err := db.Connection.Exec(c, q, state, sever, source, t, eventType, cause,
		addInf, addTxt, moID, specProb, notID, usertext, moi, moc)

	return err
}

func (db *PGCon) ClearAlertNGSA(c context.Context, name string) error {
	t := time.Now()
	// nolint:gocritic
	// language=PostgreSQL
	q := `UPDATE alarm_for_ngsa 
		SET
			state = 'CLEAR', 
			cleartime = $1
		WHERE "notificationIdentifier" = $2
`
	_, err := db.Connection.Exec(c, q, t, name)

	return err
}
