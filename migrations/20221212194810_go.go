package migrations

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

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
	type UpdateStruct struct {
		Id              uuid.UUID
		HalfSLADeadline time.Time
	}
	rows, queryErr := tx.Query("" +
		"select " +
		"id, " +
		"time time_start, " +
		"(content -> 'State' -> vs.step_name -> 'sla') sla " +
		"from variable_storage vs " +
		"where vs.status = 'running' and (content -> 'State' -> vs.step_name -> 'sla') is not null")
	if queryErr != nil {
		return queryErr
	}
	var resultRows []UpdateStruct
	for rows.Next() {
		var resultRow ResultRowStruct
		var halfSLADeadline time.Time
		scanErr := rows.Scan(&resultRow.Id, &resultRow.TimeStart, &resultRow.SLA)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		halfSLADeadline = pipeline.ComputeMaxDate(resultRow.TimeStart, float32(resultRow.SLA)/2)
		resultRows = append(resultRows, UpdateStruct{
			Id:              resultRow.Id,
			HalfSLADeadline: halfSLADeadline,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return rowsErr
	}
	rows.Close()

	for _, row := range resultRows {
		_, execErr := tx.Exec("update variable_storage set half_sla_deadline = $1 where id = $2", row.HalfSLADeadline, row.Id)
		if execErr != nil {
			return execErr
		}
	}
	return nil
}

func downGo(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
