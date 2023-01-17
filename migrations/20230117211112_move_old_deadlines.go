package migrations

import (
	"database/sql"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"time"
)

func init() {
	goose.AddMigration(upMoveOldDeadlines, downMoveOldDeadlines)
}

func upMoveOldDeadlines(tx *sql.Tx) error {
	var query string
	type ResultRowStruct struct {
		BlockId         uuid.UUID
		HalfSlaDeadline *time.Time
		SlaDeadline     *time.Time
		Already         bool
	}
	type UpdateStruct struct {
		BlockId  uuid.UUID
		Deadline time.Time
		Action   entity.TaskUpdateAction
	}
	var resultRows []UpdateStruct
	query = "select id block_id, half_sla_deadline, sla_deadline, (case when sla_deadline > NOW() THEN False ELSE True END) already from variable_storage where status in ('running', 'idle')"
	rows, queryErr := tx.Query(query)
	if queryErr != nil {
		return queryErr
	}
	for rows.Next() {
		var resultRow ResultRowStruct
		scanErr := rows.Scan(&resultRow.BlockId, &resultRow.HalfSlaDeadline, &resultRow.SlaDeadline, &resultRow.Already)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		if !resultRow.Already && resultRow.HalfSlaDeadline != nil {
			var updateRow UpdateStruct
			updateRow.Action = entity.TaskUpdateActionHalfSLABreach
			updateRow.BlockId = resultRow.BlockId
			updateRow.Deadline = *resultRow.HalfSlaDeadline
			resultRows = append(resultRows, updateRow)
		}

		if resultRow.SlaDeadline != nil {
			var updateRow UpdateStruct
			updateRow.Action = entity.TaskUpdateActionSLABreach
			updateRow.BlockId = resultRow.BlockId
			updateRow.Deadline = *resultRow.SlaDeadline
			resultRows = append(resultRows, updateRow)
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return rowsErr
	}
	rows.Close()

	query = "insert into deadlines(block_id, deadline, action) values ($1, $2, $3)"
	for _, row := range resultRows {
		_, execErr := tx.Exec(query, row.BlockId, row.Deadline, row.Action)
		if execErr != nil {
			return execErr
		}
	}
	commitErr := tx.Commit()
	if commitErr != nil {
		return commitErr
	}
	return nil
}

func downMoveOldDeadlines(tx *sql.Tx) error {
	var query string
	type ResultRowStruct struct {
		Id       uuid.UUID
		Deadline time.Time
		Action   entity.TaskUpdateAction
	}
	type UpdateStruct struct {
		Id              uuid.UUID
		HalfSlaDeadline *time.Time
		SlaDeadline     *time.Time
	}

	var resultRows []UpdateStruct
	query = "select block_id id, deadline, action from deadlines"

	rows, queryErr := tx.Query(query)
	if queryErr != nil {
		return queryErr
	}

	for rows.Next() {
		var resultRow ResultRowStruct
		var updateRow UpdateStruct
		scanErr := rows.Scan(&resultRow.Id, &resultRow.Deadline, &resultRow.Action)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		if resultRow.Action == entity.TaskUpdateActionHalfSLABreach {
			updateRow.HalfSlaDeadline = &resultRow.Deadline
		} else {
			updateRow.SlaDeadline = &resultRow.Deadline
		}
		updateRow.Id = resultRow.Id
		resultRows = append(resultRows, updateRow)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return rowsErr
	}
	rows.Close()

	for _, row := range resultRows {
		if row.HalfSlaDeadline != nil {
			query = "update variable_storage set half_sla_deadline = $1 where id = $2"
			_, queryErr := tx.Query(query, row.HalfSlaDeadline, row.Id)
			if queryErr != nil {
				return queryErr
			}
		} else {
			query = "update variable_storage set sla_deadline = $1 where id = $2"
			_, queryErr := tx.Query(query, row.HalfSlaDeadline, row.Id)
			if queryErr != nil {
				return queryErr
			}
		}
	}
	commitErr := tx.Commit()
	if commitErr != nil {
		return commitErr
	}
	return nil
}
