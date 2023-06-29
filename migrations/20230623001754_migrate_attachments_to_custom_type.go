package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iancoleman/orderedmap"
	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func init() {
	goose.AddMigration(upMigrateAttachmentsToCustomType, downMigrateAttachmentsToCustomType)
}

const (
	attachmentPrefix = "attachment:"
)

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

func GetAttachmentsFromBody(body orderedmap.OrderedMap, fields []string) map[string][]string {
	aa := make(map[string][]string)

	ff := make(map[string]struct{})
	for _, f := range fields {
		ff[strings.Trim(f, ".")] = struct{}{}
	}

	iter := func(body orderedmap.OrderedMap) {
		for _, k := range body.Keys() {
			if _, ok := ff[k]; !ok {
				continue
			}
			v, _ := body.Get(k)
			switch val := v.(type) {
			case string:
				aa[k] = []string{strings.TrimPrefix(val, attachmentPrefix)}
			case []interface{}:
				a := make([]string, 0)
				for _, item := range val {
					if attachment, ok := item.(string); ok {
						a = append(a, strings.TrimPrefix(attachment, attachmentPrefix))
					}
				}
				aa[k] = a
			}
		}
	}
	iter(body)
	return aa
}

func NewGetAttachmentsFromBody(body orderedmap.OrderedMap, fields []string) map[string][]entity.Attachment {
	aa := make(map[string][]entity.Attachment)

	ff := make(map[string]struct{})
	for _, f := range fields {
		ff[strings.Trim(f, ".")] = struct{}{}
	}

	iter := func(body orderedmap.OrderedMap) {
		for _, k := range body.Keys() {
			if _, ok := ff[k]; !ok {
				continue
			}
			v, _ := body.Get(k)
			switch val := v.(type) {
			case orderedmap.OrderedMap:
				attachmentId, ok := val.Get("id")
				if !ok {
					continue
				}
				attachmentIdString, ok := attachmentId.(string)
				if !ok {
					continue
				}
				aa[k] = []entity.Attachment{{Id: attachmentIdString}}
			case []interface{}:
				a := make([]entity.Attachment, 0)
				for _, item := range val {
					if _, ok := item.(orderedmap.OrderedMap); ok {
						orderedMapVal := item.(orderedmap.OrderedMap)
						attachmentId, ok := orderedMapVal.Get("id")
						if !ok {
							continue
						}
						attachmentIdString, ok := attachmentId.(string)
						if !ok {
							continue
						}
						a = append(a, entity.Attachment{Id: attachmentIdString})
					}
				}
				aa[k] = a
			}
		}
	}
	iter(body)
	return aa
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
			return scanErr
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
		attachments := GetAttachmentsFromBody(task.InitialApplication.ApplicationBody, task.InitialApplication.AttachmentFields)

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
			return scanErr
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

	for _, task := range resultRows {
		attachments := NewGetAttachmentsFromBody(task.InitialApplication.ApplicationBody, task.InitialApplication.AttachmentFields)

		for applicationBodyKey, attachmentKeys := range attachments {
			newAttachments := make([]string, 0)
			for key := range attachmentKeys {
				newAttachments = append(newAttachments, fmt.Sprintf("attachment:%s", attachmentKeys[key].Id))
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
