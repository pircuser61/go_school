package migrations

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func init() {
	goose.AddMigration(upGo, downGo)
}

func upGo(tx *sql.Tx) error {
	type ResultRowStruct struct {
		Id        uuid.UUID
		TimeStart time.Time
		SLA       int
	}
	rows, err := tx.Query("" +
		"select " +
		"id, " +
		"time time_start, " +
		"(content -> 'state' -> vs.step_name -> 'sla') sla " +
		"from variable_storage vs " +
		"where vs.status = 'running' and (content -> 'state' -> vs.step_name -> 'sla') is not null")
	if err != nil {
		return err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
		}
	}(rows)

	for rows.Next() {
		var resultRow ResultRowStruct
		var halfSLADeadline string
		err := rows.Scan(&resultRow.Id, &resultRow.TimeStart, &resultRow.SLA)
		if err != nil {
			return err
		}
		halfSLADeadline = pipeline.ComputeDeadline(resultRow.TimeStart, resultRow.SLA/2)

		_, err = tx.Exec("update variable_storage set half_sla_deadline = $1 where id = $2", halfSLADeadline, resultRow.Id)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func downGo(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
