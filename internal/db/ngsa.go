package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (db *PGConnection) ActiveAlertNGSA(c context.Context, sever int,
	state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error {
	t := time.Now()
	id := uuid.New().String()
	q := `INSERT INTO pipeliner.ngsa_alert(id,
                                 state,
                                 "perceivedSeverity",
                                 "eventSource",
                                 "eventTime",
                                 "eventType",
                                 "probableCause",
                                 "additionalInformation",
                                 "additionalText",
                                 "moIdentifier",
                                 "specificProblem",
                                 "notificationIdentifier",
                                 "userText",
                                 managedobjectinstance,
                                 managedobjectclass)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);`
	_, err := db.Pool.Exec(c, q, id, state, sever, source, t, eventType, cause,
		addInf, addTxt, moID, specProb, notID, usertext, moi, moc)

	return err
}

func (db *PGConnection) ClearAlertNGSA(c context.Context, name string) error {
	t := time.Now()
	q := `UPDATE pipeliner.ngsa_alert SET
	state = 'CLEAR', cleartime = $1
	WHERE "notificationIdentifier" = $2
`
	_, err := db.Pool.Exec(c, q, t, name)

	return err
}
