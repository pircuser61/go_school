package migrations

import (
	"database/sql"
	"encoding/json"
	"strings"

	//nolint:revive //need to connect to db
	_ "github.com/lib/pq"

	"github.com/google/uuid"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const updateQ = `
	UPDATE versions 
	SET content = jsonb_set(content, ('{pipeline, blocks, ' || $1 || ', output}')::text[], $2::jsonb, true)
	WHERE id = $3
	`

const selectQ = `SELECT id, content->>'pipeline' FROM versions WHERE content->>'pipeline' IS NOT NULL`

type pipelineType struct {
	Entrypoint string `json:"entrypoint"`
	Blocks     map[string]struct {
		X          int                         `json:"x,omitempty"`
		Y          int                         `json:"y,omitempty"`
		TypeID     string                      `json:"type_id" example:"approver"`
		BlockType  string                      `json:"block_type" enums:"python3,go,internal,term,scenario" example:"python3"`
		Title      string                      `json:"title" example:"lock-bts"`
		ShortTitle string                      `json:"short_title,omitempty" example:"lock-bts"`
		Input      []entity.EriusFunctionValue `json:"input,omitempty"`
		Output     []entity.EriusFunctionValue `json:"output,omitempty"`
		ParamType  string                      `json:"param_type,omitempty"`
		Params     json.RawMessage             `json:"params,omitempty" swaggertype:"object"`
		Next       map[string][]string         `json:"next,omitempty"`
		Sockets    []entity.Socket             `json:"sockets,omitempty"`
	} `json:"blocks"`
}

//nolint:gochecknoinits //необходимо для гуся
func init() {
	goose.AddMigration(upJSONSchemaOutputs, downJSONSchemaOutputs)
}

func upJSONSchemaOutputs(tx *sql.Tx) error {
	rows, queryErr := tx.Query(selectQ)
	if queryErr != nil {
		return queryErr
	}
	defer rows.Close()

	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	pipelines := map[uuid.UUID]string{}

	for rows.Next() {
		var (
			versionID      uuid.UUID
			pipelineString string
		)

		scanErr := rows.Scan(
			&versionID,
			&pipelineString,
		)
		if scanErr != nil {
			return scanErr
		}

		pipelines[versionID] = pipelineString
	}

	for versionID, pipelineString := range pipelines {
		var pipeline pipelineType

		err := json.Unmarshal([]byte(pipelineString), &pipeline)
		if err != nil {
			return err
		}

		for blockName := range pipeline.Blocks {
			if strings.TrimSpace(blockName) == "" || pipeline.Blocks[blockName].Output == nil {
				continue
			}

			output := &script.JSONSchema{
				Type:       "object",
				Properties: map[string]script.JSONSchemaPropertiesValue{},
			}

			for _, param := range pipeline.Blocks[blockName].Output {
				paramValue := script.JSONSchemaPropertiesValue{
					Type:   param.Type,
					Global: param.Global,
					Format: param.Format,
				}

				if strings.EqualFold(param.Format, "ssoperson") {
					paramValue.Properties = people.GetSsoPersonSchemaProperties()
				}

				if param.Type == "array" && (param.Name == "attachments" || strings.EqualFold(param.Format, "file")) {
					paramValue.Items = &script.ArrayItems{
						Type:   "string",
						Format: "file",
					}
				}

				output.Properties[param.Name] = paramValue
			}

			_, execErr := tx.Exec(updateQ, blockName, output, versionID)
			if execErr != nil {
				return execErr
			}
		}
	}

	return nil
}

func downJSONSchemaOutputs(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	rows, queryErr := tx.Query(selectQ)
	if queryErr != nil {
		return queryErr
	}

	defer rows.Close()

	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	pipelines := map[uuid.UUID]string{}

	for rows.Next() {
		var (
			versionID      uuid.UUID
			pipelineString string
		)

		scanErr := rows.Scan(
			&versionID,
			&pipelineString,
		)

		if scanErr != nil {
			return scanErr
		}

		pipelines[versionID] = pipelineString
	}

	for versionID, pipelineString := range pipelines {
		var pipeline entity.PipelineType

		err := json.Unmarshal([]byte(pipelineString), &pipeline)
		if err != nil {
			return err
		}

		for blockName := range pipeline.Blocks {
			if strings.TrimSpace(blockName) == "" || pipeline.Blocks[blockName].Output == nil {
				continue
			}

			var output []entity.EriusFunctionValue

			//nolint:gocritic //коллекции на здоровые структуры без поинтера, классика
			for name, param := range pipeline.Blocks[blockName].Output.Properties {
				output = append(output, entity.EriusFunctionValue{
					Name:   name,
					Type:   param.Type,
					Global: param.Global,
					Format: param.Format,
				})
			}

			_, execErr := tx.Exec(updateQ, blockName, output, versionID)
			if execErr != nil {
				return execErr
			}
		}
	}

	return nil
}
