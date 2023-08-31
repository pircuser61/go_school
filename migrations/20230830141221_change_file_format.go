package migrations

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func init() {
	goose.AddMigration(upChangeFileFormat, downChangeFileFormat)
}

func upChangeFileFormat(tx *sql.Tx) error {
	q := `Select id, content->>'State' from variable_storage where  content -> 'State' is not null `
	type resultStruct struct {
		resultMap map[string]json.RawMessage
		id        string
	}
	var result []resultStruct
	rows, queryErr := tx.Query(q)
	if queryErr != nil {
		return queryErr
	}
	defer rows.Close()
	for rows.Next() {
		resultMap := map[string]json.RawMessage{}
		resultState := map[string]json.RawMessage{}
		var state string
		var Id string

		scanErr := rows.Scan(
			&Id,
			&state,
		)
		if scanErr != nil {
			return scanErr
		}

		err := json.Unmarshal([]byte(state), &resultState)
		if err != nil {
			return err
		}
		for key, val := range resultState {
			var data interface{}

			switch {
			case strings.Contains(key, "approver"):
				data = &pipeline.ApproverData{}

			case strings.Contains(key, "execution"):
				data = &pipeline.ExecutionData{}

			case strings.Contains(key, "sign"):
				data = &pipeline.SignData{}

			case strings.Contains(key, "form"):
				data = &pipeline.FormData{}

			case strings.Contains(key, "function"):
				data = &pipeline.ExecutableFunction{}
			}
			if data != nil {
				err := json.Unmarshal(val, &data)
				if err != nil {
					return err
				}

				resJson, mErr := json.Marshal(data)
				if mErr != nil {
					return mErr
				}
				resultMap[key] = resJson
			} else {
				resultMap[key] = val
			}
		}
		result = append(result, resultStruct{
			resultMap: resultMap,
			id:        Id,
		})
	}
	for key := range result {
		insertStateQ := `Update variable_storage set content = jsonb_set(content,'{State}', $1, false) where id = $2`
		_, execErr := tx.Exec(insertStateQ, result[key].resultMap, result[key].id)
		if execErr != nil {
			return execErr
		}
	}
	return nil
}

func downChangeFileFormat(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
