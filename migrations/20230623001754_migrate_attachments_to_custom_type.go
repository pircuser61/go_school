package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

func init() {
	goose.AddMigration(upMigrateAttachmentsToCustomType, downMigrateAttachmentsToCustomType)
}

func GetUUIDFromAttachmentString(attachmentString string) (string, error) {
	splittedAttachment := strings.Split(attachmentString, ":")

	if len(splittedAttachment) == 1 {
		return splittedAttachment[0], nil
	} else if len(splittedAttachment) == 2 {
		return splittedAttachment[1], nil
	} else {
		return "", fmt.Errorf("dont know how to parse: %s", attachmentString)
	}
}

func upMigrateAttachmentsToCustomType(tx *sql.Tx) error {
	querySelect := "select id, run_context -> 'initial_application' from works where (works.run_context -> 'initial_application' ->> 'attachment_fields') != ''"
	queryUpdate := "update works set run_context = jsonb_set(run_context, '{initial_application,application_body,{{key}}}', $1::jsonb) where id  = $2"

	rows, queryErr := tx.Query(querySelect)

	if queryErr != nil {
		return queryErr
	}
	defer rows.Close()

	type ResultRow struct {
		Id                 string                    `json:"id"`
		InitialApplication entity.InitialApplication `json:"initial_application"`
	}
	var resultRows []ResultRow

	for rows.Next() {
		var id string
		var initialApplication []uint8
		var resultRow ResultRow

		scanErr := rows.Scan(&id, &initialApplication)
		if scanErr != nil {
			panic(scanErr)
		}

		unmarshalErr := json.Unmarshal(initialApplication, &resultRow.InitialApplication)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		resultRow.Id = id

		resultRows = append(resultRows, resultRow)

	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}
	rows.Close()

	type NewAttachment struct {
		Id string `json:"id"`
	}
	for _, task := range resultRows {
		attachments := mail.GetAttachmentsFromBody(task.InitialApplication.ApplicationBody, task.InitialApplication.AttachmentFields)
		if len(attachments) < 2 || len(task.InitialApplication.AttachmentFields) < 2 {
			continue
		}

		for applicationBodyKey, attachmentKeys := range attachments {
			var newAttachments []NewAttachment
			for key := range attachmentKeys {
				attachmentUUID, getErr := GetUUIDFromAttachmentString(attachmentKeys[key])
				if getErr != nil {
					return getErr
				}
				newAttachments = append(newAttachments, NewAttachment{Id: attachmentUUID})
			}

			attachmentsBytes, marshalErr := json.Marshal(newAttachments)
			if marshalErr != nil {
				return marshalErr
			}

			_, execErr := tx.Exec(strings.Replace(queryUpdate, "{{key}}", applicationBodyKey, 1), attachmentsBytes, task.Id)
			if execErr != nil {
				return execErr
			}
		}
	}
	return nil
}

func downMigrateAttachmentsToCustomType(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
