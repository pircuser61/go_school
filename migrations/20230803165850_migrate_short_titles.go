package migrations

import (
	"database/sql"
	"encoding/json"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func init() {
	goose.AddMigration(upMigrateShortTitles, downMigrateShortTitles)
}

func upMigrateShortTitles(tx *sql.Tx) error {
	// This code is executed when the migration is applied.
	type FormParams struct {
		SchemaName string `json:"schema_name"`
	}

	q := `Select id, content from versions`

	rows, queryErr := tx.Query(q)
	if queryErr != nil {
		return queryErr
	}

	for rows.Next() {
		var resultRow entity.EriusScenario
		scanErr := rows.Scan(
			&resultRow.ID,
			&resultRow.Pipeline,
		)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}

		for _, val := range resultRow.Pipeline.Blocks {
			if val.TypeID == "approver" && val.ShortTitle == "" {
				val.ShortTitle = "Нода согласование"
			}
			if val.TypeID == "execution" && val.ShortTitle == "" {
				val.ShortTitle = "Нода исполнение"
			}
			if val.TypeID == "form" {
				var params FormParams
				err := json.Unmarshal(val.Params, &params)
				if err != nil {
					return err
				}
				if params.SchemaName != "" {
					val.ShortTitle = params.SchemaName
				} else {
					if val.ShortTitle == "" {
						val.ShortTitle = "Нода Форма"
					}
				}
			}
		}
		insertQ := `Update versions set content = jsonb_set(content,'{pipeline}', $1, false) where id = $2`
		_, execErr := tx.Exec(insertQ, resultRow.Pipeline, resultRow.ID)
		if execErr != nil {
			return execErr
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return rowsErr
	}
	rows.Close()
	return nil
}

func downMigrateShortTitles(tx *sql.Tx) error {
	type FormParams struct {
		SchemaName string `json:"schema_name"`
	}

	q := `Select id, content->>'pipeline' from versions`

	rows, queryErr := tx.Query(q)
	if queryErr != nil {
		return queryErr
	}
	var content string
	for rows.Next() {
		var resultRow entity.EriusScenario
		scanErr := rows.Scan(
			&resultRow.ID,
			&content,
		)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		err := json.Unmarshal([]byte(content), &resultRow.Pipeline)
		if err != nil {
			return err
		}

		for _, val := range resultRow.Pipeline.Blocks {
			if val.TypeID == "approver" && val.ShortTitle == "Нода согласование" {
				val.ShortTitle = ""
			}
			if val.TypeID == "execution" && val.ShortTitle == "Нода исполнение" {
				val.ShortTitle = ""
			}
			if val.TypeID == "form" {
				var params FormParams
				err := json.Unmarshal(val.Params, &params)
				if err != nil {
					return err
				}
				if val.ShortTitle == "Нода Форма" {
					val.ShortTitle = ""
				}
			}
		}

		insertQ := `Update versions set content = jsonb_set(content,'{pipeline}', $1, false) where id = $2`
		_, execErr := tx.Exec(insertQ, &resultRow.Pipeline, resultRow.ID)
		if execErr != nil {
			return execErr
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return rowsErr
	}
	rows.Close()
	return nil
}
