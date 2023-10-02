package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"strings"
)

func init() {
	goose.AddMigration(upMembers, downMembers)
}

type member struct {
	Id       string `json:"id"`
	BlockId  string `json:"block_id"`
	Login    string `json:"login"`
	Finished bool   `json:"finished"`
	IsActed  bool   `json:"is_acted"`
}

type membersFormData struct {
	Executors      map[string]struct{} `json:"executors"`
	IsFilled       bool                `json:"is_filled"`
	ActualExecutor *string             `json:"actual_executor,omitempty"`
}

func (m *membersFormData) getMembers(blockId string) (res []member) {
	if m.IsFilled {
		for login := range m.Executors {
			res = append(res, member{
				Id:       uuid.New().String(),
				BlockId:  blockId,
				Login:    login,
				Finished: true,
				IsActed:  true,
			})
		}
	}

	return
}

type membersSignData struct {
	Decision     *string        `json:"decision,omitempty"`
	ActualSigner *string        `json:"actual_signer,omitempty"`
	SignLog      []signLogEntry `json:"sign_log,omitempty"`
}

type signLogEntry struct {
	Login string `json:"login"`
}

func (m *membersSignData) getMembers(blockId string) (res []member) {
	for i := range m.SignLog {
		res = append(res, member{
			Id:       uuid.New().String(),
			BlockId:  blockId,
			Login:    m.SignLog[i].Login,
			Finished: true,
			IsActed:  true,
		})
	}

	return res
}

type membersExecutionData struct {
	Decision                 *string                   `json:"decision,omitempty"`
	ActualExecutor           *string                   `json:"actual_executor,omitempty"`
	EditingAppLog            []executorEditApp         `json:"editing_app_log,omitempty"`
	ChangedExecutorsLogs     []changeExecutorsLogs     `json:"change_executors_logs,omitempty"`
	RequestExecutionInfoLogs []requestExecutionInfoLog `json:"request_execution_info_logs,omitempty"`
}

type requestExecutionInfoLog struct {
	Login   string `json:"login"`
	ReqType string `json:"req_type"`
}

type executorEditApp struct {
	Executor string `json:"executor"`
}

type changeExecutorsLogs struct {
	OldLogin string `json:"old_login"`
	NewLogin string `json:"new_login"`
}

func (m *membersExecutionData) getMembers(blockId string) (res []member) {
	if m.Decision != nil {
		for i := range m.RequestExecutionInfoLogs {
			if m.RequestExecutionInfoLogs[i].ReqType == "question" {
				res = append(res, member{
					Id:       uuid.New().String(),
					BlockId:  blockId,
					Login:    m.RequestExecutionInfoLogs[i].Login,
					Finished: true,
					IsActed:  true,
				})
			}
		}

		for i := range m.ChangedExecutorsLogs {
			if m.RequestExecutionInfoLogs[i].ReqType == "question" {
				res = append(res, member{
					Id:       uuid.New().String(),
					BlockId:  blockId,
					Login:    m.ChangedExecutorsLogs[i].OldLogin,
					Finished: true,
					IsActed:  true,
				})
			}
		}
	}

	return
}

type membersApproverData struct {
	Decision            *string              `json:"decision,omitempty"`
	ApproverLog         []approverLogEntry   `json:"approver_log,omitempty"`
	AdditionalApprovers []additionalApprover `json:"additional_approvers"`
}

type approverLogEntry struct {
	Login string `json:"login"`
}

type additionalApprover struct {
	ApproverLogin     string  `json:"approver_login"`
	BaseApproverLogin string  `json:"base_approver_login"`
	Decision          *string `json:"decision"`
}

func (m *membersApproverData) getMembers(blockId string) (res []member) {
	if m.Decision != nil {
		for i := range m.ApproverLog {
			if m.ApproverLog[i].Login == "" {
				continue
			}
			res = append(res, member{
				Id:       uuid.New().String(),
				BlockId:  blockId,
				Login:    m.ApproverLog[i].Login,
				Finished: true,
				IsActed:  true,
			})
		}

		for i := range m.AdditionalApprovers {
			if m.AdditionalApprovers[i].ApproverLogin == "" ||
				m.AdditionalApprovers[i].Decision == nil {
				continue
			}
			res = append(res, member{
				Id:       uuid.New().String(),
				BlockId:  blockId,
				Login:    m.AdditionalApprovers[i].ApproverLogin,
				Finished: true,
				IsActed:  true,
			})

			res = append(res, member{
				Id:       uuid.New().String(),
				BlockId:  blockId,
				Login:    m.AdditionalApprovers[i].BaseApproverLogin,
				Finished: true,
				IsActed:  true,
			})
		}
	}

	return
}

type memberExtractor interface {
	getMembers(blockId string) (res []member)
}

func upMembers(tx *sql.Tx) error {
	const q = `
		select 
			w.id,
			vs.content->>'State'
		from members w
		left join variable_storage vs on vs.work_id = w.id
		where w.status in (2, 4, 6) and 
			vs.content is not null and
			vs.time = (select max(time) from variable_storage 
						where work_id = w.id)
	`

	rows, queryErr := tx.Query(q)
	if queryErr != nil {
		return queryErr
	}
	defer rows.Close()

	var members = make([]member, 0, 20)

	for rows.Next() {
		workState := map[string]json.RawMessage{}
		var state, workId, stepName string

		if scanErr := rows.Scan(&workId, &state, &stepName); scanErr != nil {
			return scanErr
		}

		if err := json.Unmarshal([]byte(state), &workState); err != nil {
			return err
		}

		for key, val := range workState {
			var data memberExtractor

			switch {
			case strings.Contains(key, "approver_"):
				data = &membersApproverData{}

			case strings.Contains(key, "execution_"):
				data = &membersExecutionData{}

			case strings.Contains(key, "sign_"):
				data = &membersSignData{}

			case strings.Contains(key, "form_"):
				data = &membersFormData{}
			}
			if data != nil {
				if err := json.Unmarshal(val, &data); err != nil {
					fmt.Println("json.Unmarshal error, block_id:", workId)
					return err
				}

				const getBlockId = `
				select id from variable_storage where work_id = $1 and 
					step_name = $2
					time = (select max(time) 
						from variable_storage 
						where work_id = $1 and step_name = $2
				)`

				blockId := ""
				row := tx.QueryRow(getBlockId, workId, key)
				if err := row.Scan(&blockId); err != nil {
					return err
				}

				members = append(members, data.getMembers(blockId)...)
			} else {
				fmt.Printf("key: %s is not recognized \n", key)
			}
		}
	}

	members = uniqMembers(members)

	for i := range members {
		insert := `
			insert into members(id, block_id, login, finished, actions, is_acted) 
				values($1, $2, $3, $4, '{}', $5)
			on conflict do nothing
		`

		_, execErr := tx.Exec(
			insert,
			members[i].Id,
			members[i].BlockId,
			members[i].Login,
			members[i].Finished,
			members[i].IsActed,
		)
		if execErr != nil {
			return execErr
		}
	}
	return nil
}

func uniqMembers(in []member) (out []member) {
	mapMembers := make(map[string]interface{})

	for i := range in {
		if _, exists := mapMembers[in[i].BlockId+in[i].Login]; !exists {
			out = append(out, in[i])
			mapMembers[in[i].BlockId+in[i].Login] = true
		}
	}

	return out
}

func downMembers(tx *sql.Tx) error {
	return nil
}
