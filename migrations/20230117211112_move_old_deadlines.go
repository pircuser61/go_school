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
	type ResultRowStruct struct {
		BlockID         uuid.UUID
		HalfSlaDeadline *time.Time
		SlaDeadline     *time.Time
		CheckHalfSLA    bool
		CheckSLA        bool
	}

	type UpdateStruct struct {
		BlockID  uuid.UUID
		Deadline time.Time
		Action   entity.TaskUpdateAction
	}

	var (
		resultRows []UpdateStruct
		query      string
	)

	query = `
		select id block_id, half_sla_deadline, sla_deadline, check_half_sla, check_sla 
			from variable_storage where status in ('running', 'idle')`

	rows, queryErr := tx.Query(query)
	if queryErr != nil {
		return queryErr
	}

	defer rows.Close()

	for rows.Next() {
		var resultRow ResultRowStruct

		scanErr := rows.Scan(
			&resultRow.BlockID,
			&resultRow.HalfSlaDeadline,
			&resultRow.SlaDeadline,
			&resultRow.CheckHalfSLA,
			&resultRow.CheckSLA,
		)
		if scanErr != nil {
			rows.Close()

			return scanErr
		}

		if resultRow.CheckHalfSLA && resultRow.HalfSlaDeadline != nil {
			resultRows = append(resultRows, UpdateStruct{
				BlockID:  resultRow.BlockID,
				Deadline: *resultRow.HalfSlaDeadline,
				Action:   entity.TaskUpdateActionHalfSLABreach,
			})
		}

		if resultRow.CheckSLA && resultRow.SlaDeadline != nil {
			resultRows = append(resultRows, UpdateStruct{
				BlockID:  resultRow.BlockID,
				Deadline: *resultRow.SlaDeadline,
				Action:   entity.TaskUpdateActionSLABreach,
			})
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()

		return rowsErr
	}

	query = "insert into deadlines(id, block_id, deadline, action) values ($1, $2, $3, $4)"

	for _, row := range resultRows {
		id := uuid.New()

		_, execErr := tx.Exec(query, id, row.BlockID, row.Deadline, row.Action)
		if execErr != nil {
			return execErr
		}
	}

	return nil
}

func downMoveOldDeadlines(tx *sql.Tx) error {
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

	var (
		resultRows []UpdateStruct
		query      string
	)

	query = "select block_id id, deadline, action from deadlines"

	rows, queryErr := tx.Query(query)
	if queryErr != nil {
		return queryErr
	}

	for rows.Next() {
		var (
			resultRow ResultRowStruct
			updateRow UpdateStruct
		)

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
			query = "update variable_storage set half_sla_deadline = $1, check_half_sla = True where id = $2"

			_, queryErr := tx.Exec(query, row.HalfSlaDeadline, row.Id)
			if queryErr != nil {
				return queryErr
			}
		} else {
			query = "update variable_storage set sla_deadline = $1, check_sla = True where id = $2"

			_, queryErr := tx.Exec(query, row.SlaDeadline, row.Id)
			if queryErr != nil {
				return queryErr
			}
		}
	}

	return nil
}
