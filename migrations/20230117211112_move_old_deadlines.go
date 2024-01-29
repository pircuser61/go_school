package migrations

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

//nolint:gochecknoinits //необходимо для гуся
func init() {
	goose.AddMigration(upMoveOldDeadlines, downMoveOldDeadlines)
}

func upMoveOldDeadlines(tx *sql.Tx) error {
	type ResultRowStruct struct {
		BlockID         uuid.UUID
		HalfSLADeadline *time.Time
		SLADeadline     *time.Time
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
			&resultRow.HalfSLADeadline,
			&resultRow.SLADeadline,
			&resultRow.CheckHalfSLA,
			&resultRow.CheckSLA,
		)
		if scanErr != nil {
			rows.Close()

			return scanErr
		}

		if resultRow.CheckHalfSLA && resultRow.HalfSLADeadline != nil {
			resultRows = append(resultRows, UpdateStruct{
				BlockID:  resultRow.BlockID,
				Deadline: *resultRow.HalfSLADeadline,
				Action:   entity.TaskUpdateActionHalfSLABreach,
			})
		}

		if resultRow.CheckSLA && resultRow.SLADeadline != nil {
			resultRows = append(resultRows, UpdateStruct{
				BlockID:  resultRow.BlockID,
				Deadline: *resultRow.SLADeadline,
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
		ID       uuid.UUID
		Deadline time.Time
		Action   entity.TaskUpdateAction
	}

	type UpdateStruct struct {
		ID              uuid.UUID
		HalfSLADeadline *time.Time
		SLADeadline     *time.Time
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

	defer rows.Close()

	for rows.Next() {
		var (
			resultRow ResultRowStruct
			updateRow UpdateStruct
		)

		scanErr := rows.Scan(&resultRow.ID, &resultRow.Deadline, &resultRow.Action)

		if scanErr != nil {
			return scanErr
		}

		if resultRow.Action == entity.TaskUpdateActionHalfSLABreach {
			updateRow.HalfSLADeadline = &resultRow.Deadline
		} else {
			updateRow.SLADeadline = &resultRow.Deadline
		}

		updateRow.ID = resultRow.ID
		resultRows = append(resultRows, updateRow)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	for _, row := range resultRows {
		if row.HalfSLADeadline != nil {
			query = "update variable_storage set half_sla_deadline = $1, check_half_sla = True where id = $2"

			_, queryErr := tx.Exec(query, row.HalfSLADeadline, row.ID)
			if queryErr != nil {
				return queryErr
			}
		} else {
			query = "update variable_storage set sla_deadline = $1, check_sla = True where id = $2"

			_, queryErr := tx.Exec(query, row.SLADeadline, row.ID)
			if queryErr != nil {
				return queryErr
			}
		}
	}

	return nil
}
