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

const (
	approverType  = "approver"
	executionType = "execution"
	formType      = "form"

	approverName  = "Нода согласование"
	executionName = "Нода исполнение"
	formName      = "Нода Форма"

	insertQ = `Update versions set content = jsonb_set(content,'{pipeline}', $1, false) where id = $2`
)

// nolint
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
	defer rows.Close()
	for rows.Next() {
		var resultRow entity.EriusScenario
		scanErr := rows.Scan(
			&resultRow.ID,
			&resultRow.Pipeline,
		)
		if scanErr != nil {
			return scanErr
		}

		for i := range resultRow.Pipeline.Blocks {
			val := resultRow.Pipeline.Blocks[i]
			switch val.TypeID {
			case approverType:
				if val.ShortTitle == "" {
					val.ShortTitle = approverName
				}
			case executionType:
				if val.ShortTitle == "" {
					val.ShortTitle = executionName
				}
			case formType:
				{
					var params FormParams
					err := json.Unmarshal(val.Params, &params)
					if err != nil {
						return err
					}
					if params.SchemaName != "" {
						val.ShortTitle = params.SchemaName
					} else if val.ShortTitle == "" {
						val.ShortTitle = formName
					}
				}
			}
		}
		_, execErr := tx.Exec(insertQ, resultRow.Pipeline, resultRow.ID)
		if execErr != nil {
			return execErr
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}
	return nil
}

// nolint
func downMigrateShortTitles(tx *sql.Tx) error {
	type FormParams struct {
		SchemaName string `json:"schema_name"`
	}

	q := `Select id, content->>'pipeline' from versions`

	rows, queryErr := tx.Query(q)
	if queryErr != nil {
		return queryErr
	}
	defer rows.Close()
	var content string
	for rows.Next() {
		var resultRow entity.EriusScenario
		scanErr := rows.Scan(
			&resultRow.ID,
			&content,
		)
		if scanErr != nil {
			return scanErr
		}
		err := json.Unmarshal([]byte(content), &resultRow.Pipeline)
		if err != nil {
			return err
		}

		for _, val := range resultRow.Pipeline.Blocks {
			switch val.TypeID {
			case approverType:
				if val.ShortTitle == approverName {
					val.ShortTitle = ""
				}
			case executionType:
				if val.ShortTitle == executionName {
					val.ShortTitle = ""
				}
			case formType:
				{
					var params FormParams
					err := json.Unmarshal(val.Params, &params)
					if err != nil {
						return err
					}
					if val.ShortTitle == formName {
						val.ShortTitle = ""
					}
				}
			default:
				continue
			}
		}

		_, execErr := tx.Exec(insertQ, &resultRow.Pipeline, resultRow.ID)
		if execErr != nil {
			return execErr
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}
	return nil
}
