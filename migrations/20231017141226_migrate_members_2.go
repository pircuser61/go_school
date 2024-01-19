package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigration(upMembers, downMembers)
}

type member struct {
	ID       string
	WorkID   string
	StepName string
	Login    string
	Finished bool
	IsActed  bool
}

type membersFormData struct {
	Executors      map[string]struct{} `json:"executors"`
	IsFilled       bool                `json:"is_filled"`
	ActualExecutor *string             `json:"actual_executor,omitempty"`
}

func (m *membersFormData) getMembers(wID, sName string) (res []member) {
	if m.IsFilled {
		for login := range m.Executors {
			res = append(res, member{
				ID:       uuid.New().String(),
				WorkID:   wID,
				StepName: sName,
				Login:    login,
				Finished: true,
				IsActed:  true,
			})
		}

		if m.ActualExecutor != nil {
			res = append(res, member{
				ID:       uuid.New().String(),
				WorkID:   wID,
				StepName: sName,
				Login:    *m.ActualExecutor,
				Finished: true,
				IsActed:  true,
			})
		}
	}

	return
}

type membersSignData struct {
	Decision     *string         `json:"decision,omitempty"`
	ActualSigner *string         `json:"actual_signer,omitempty"`
	SignLog      arrSignLogEntry `json:"sign_log,omitempty"`
}

type arrSignLogEntry []signLogEntry

type signLogEntry struct {
	Login string `json:"login"`
}

func (at *arrSignLogEntry) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []signLogEntry
		atTemp  signLogEntry
		stTemp  []string
	)

	if err := json.Unmarshal(b, &arrTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		for i := range stTemp {
			s, _ := strconv.Unquote(stTemp[i])

			err := json.Unmarshal([]byte(s), &atTemp)
			if err != nil {
				return err
			}

			*at = append(*at, atTemp)
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		return nil
	}

	*at = arrTemp

	return nil
}

func (at *signLogEntry) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Login string `json:"login"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		*at = atTemp

		return nil
	}

	*at = atTemp

	return nil
}

func (m *membersSignData) getMembers(wID, sName string) (res []member) {
	for i := range m.SignLog {
		res = append(res, member{
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
			Login:    m.SignLog[i].Login,
			Finished: true,
			IsActed:  true,
		})
	}

	return res
}

type membersExecutionData struct {
	Decision                 *string                    `json:"decision,omitempty"`
	Executors                map[string]struct{}        `json:"executors"`
	ActualExecutor           *string                    `json:"actual_executor,omitempty"`
	EditingAppLog            arrExecutorEditApp         `json:"editing_app_log,omitempty"`
	ChangedExecutorsLogs     arrChangeExecutorLog       `json:"change_executors_logs,omitempty"`
	RequestExecutionInfoLogs arrRequestExecutionInfoLog `json:"request_execution_info_logs,omitempty"`
}

type arrExecutorEditApp []executorEditApp

func (at *arrExecutorEditApp) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []executorEditApp
		atTemp  executorEditApp
		stTemp  []string
	)

	if err := json.Unmarshal(b, &arrTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		for i := range stTemp {
			s, _ := strconv.Unquote(stTemp[i])

			err := json.Unmarshal([]byte(s), &atTemp)
			if err != nil {
				return err
			}

			*at = append(*at, atTemp)
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		return nil
	}

	*at = arrTemp

	return nil
}

func (at *executorEditApp) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Executor string `json:"executor"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		*at = atTemp

		return nil
	}

	*at = atTemp

	return nil
}

type arrChangeExecutorLog []changeExecutorsLogs

func (at *arrChangeExecutorLog) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []changeExecutorsLogs
		atTemp  changeExecutorsLogs
	)

	stTemp := make([]string, 0)

	if err := json.Unmarshal(b, &arrTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return errStr
		}

		for i := range stTemp {
			stTemp[i] = strings.Trim(stTemp[i], "\"")

			var bt = []byte(stTemp[i])

			fmt.Println(bt)

			err := json.Unmarshal([]byte(stTemp[i]), &atTemp)
			if err != nil {
				return err
			}

			*at = append(*at, atTemp)
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		return nil
	}

	*at = arrTemp

	return nil
}

func (at *changeExecutorsLogs) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		OldLogin string `json:"old_login"`
		NewLogin string `json:"new_login"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		*at = atTemp

		return nil
	}

	*at = atTemp

	return nil
}

type requestExecutionInfoLog struct {
	Login   string `json:"login"`
	ReqType string `json:"req_type"`
}

type arrRequestExecutionInfoLog []requestExecutionInfoLog

func (at *arrRequestExecutionInfoLog) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []requestExecutionInfoLog
		atTemp  requestExecutionInfoLog
		stTemp  []string
	)

	if err := json.Unmarshal(b, &arrTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		for i := range stTemp {
			s, _ := strconv.Unquote(stTemp[i])

			err := json.Unmarshal([]byte(s), &atTemp)
			if err != nil {
				return err
			}

			*at = append(*at, atTemp)
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		return nil
	}

	*at = arrTemp

	return nil
}

func (at *requestExecutionInfoLog) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Login   string `json:"login"`
		ReqType string `json:"req_type"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		*at = atTemp

		return nil
	}

	*at = atTemp

	return nil
}

type executorEditApp struct {
	Executor string `json:"executor"`
}

type changeExecutorsLogs struct {
	OldLogin string `json:"old_login"`
	NewLogin string `json:"new_login"`
}

func (m *membersExecutionData) getMembers(wID, sName string) (res []member) {
	for login := range m.Executors {
		res = append(res, member{
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
			Login:    login,
			Finished: true,
			IsActed:  true,
		})
	}

	for i := range m.RequestExecutionInfoLogs {
		if m.RequestExecutionInfoLogs[i].ReqType == "question" {
			res = append(res, member{
				ID:       uuid.New().String(),
				WorkID:   wID,
				StepName: sName,
				Login:    m.RequestExecutionInfoLogs[i].Login,
				Finished: true,
				IsActed:  true,
			})
		}
	}

	for i := range m.ChangedExecutorsLogs {
		res = append(res, member{
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
			Login:    m.ChangedExecutorsLogs[i].OldLogin,
			Finished: true,
			IsActed:  true,
		})
	}

	return
}

type membersApproverData struct {
	Decision            *string               `json:"decision,omitempty"`
	Approvers           map[string]struct{}   `json:"approvers"`
	ApproverLog         arrApproverLogEntry   `json:"approver_log,omitempty"`
	AdditionalApprovers arrAdditionalApprover `json:"additional_approvers"`
}

type approverLogEntry struct {
	Login string `json:"login"`
}

type arrApproverLogEntry []approverLogEntry

func (at *arrApproverLogEntry) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []approverLogEntry
		atTemp  approverLogEntry
	)

	stTemp := make([]string, 0)

	if err := json.Unmarshal(b, &arrTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return errStr
		}

		for i := range stTemp {
			stTemp[i] = strings.Trim(stTemp[i], "\"")

			var bt = []byte(stTemp[i])

			fmt.Println(bt)

			err := json.Unmarshal([]byte(stTemp[i]), &atTemp)
			if err != nil {
				return err
			}

			*at = append(*at, atTemp)
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		return nil
	}

	*at = arrTemp

	return nil
}

func (at *approverLogEntry) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Login string `json:"login"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		*at = atTemp

		return nil
	}

	*at = atTemp

	return nil
}

type additionalApprover struct {
	ApproverLogin     string  `json:"approver_login"`
	BaseApproverLogin string  `json:"base_approver_login"`
	Decision          *string `json:"decision"`
}

type arrAdditionalApprover []additionalApprover

func (at *arrAdditionalApprover) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []additionalApprover
		atTemp  additionalApprover
		stTemp  []string
	)

	if err := json.Unmarshal(b, &arrTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		for i := range stTemp {
			s, _ := strconv.Unquote(stTemp[i])

			err := json.Unmarshal([]byte(s), &atTemp)
			if err != nil {
				return err
			}

			*at = append(*at, atTemp)
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		return nil
	}

	*at = arrTemp

	return nil
}

func (at *additionalApprover) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		ApproverLogin     string  `json:"approver_login"`
		BaseApproverLogin string  `json:"base_approver_login"`
		Decision          *string `json:"decision"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		s, _ := strconv.Unquote(string(b))

		err := json.Unmarshal([]byte(s), &atTemp)
		if err != nil {
			return err
		}

		*at = atTemp

		return nil
	}

	*at = atTemp

	return nil
}

func (m *membersApproverData) getMembers(wID, sName string) (res []member) {
	for login := range m.Approvers {
		res = append(res, member{
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
			Login:    login,
			Finished: true,
			IsActed:  true,
		})
	}

	for i := range m.ApproverLog {
		if m.ApproverLog[i].Login == "" {
			continue
		}

		res = append(res, member{
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
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
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
			Login:    m.AdditionalApprovers[i].ApproverLogin,
			Finished: true,
			IsActed:  true,
		})

		res = append(res, member{
			ID:       uuid.New().String(),
			WorkID:   wID,
			StepName: sName,
			Login:    m.AdditionalApprovers[i].BaseApproverLogin,
			Finished: true,
			IsActed:  true,
		})
	}

	return
}

type memberExtractor interface {
	getMembers(wID, sName string) (res []member)
}

func upMembers(tx *sql.Tx) error {
	const q = `
		select 
			w.id,
			vs.content->>'State'
		from works w
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

	var (
		members    = make([]member, 0)
		countWorks = 0
	)

	for rows.Next() {
		workState := map[string]json.RawMessage{}

		var state, workID string

		if scanErr := rows.Scan(&workID, &state); scanErr != nil {
			return scanErr
		}

		if err := json.Unmarshal([]byte(state), &workState); err != nil {
			return err
		}

		fmt.Println("work id: " + workID)

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
					fmt.Println("json.Unmarshal error, work id:", workID)

					return err
				}

				members = append(members, data.getMembers(workID, key)...)
			}
		}
		countWorks++
	}

	members = uniqMembers(members)

	fmt.Printf("total tasks: %d \n", countWorks)
	fmt.Printf("total members: %d \n", len(members))
	fmt.Println("inserting to members ...")

	values := make([]string, 0, len(members))

	for i := range members {
		values = append(values, fmt.Sprintf("('%s', (select vs.id from variable_storage vs where vs.work_id = '%s' and vs.step_name = '%s' limit 1), '%s', true, true)",
			members[i].ID,
			members[i].WorkID,
			members[i].StepName,
			members[i].Login,
		))
	}

	if len(values) == 0 {
		return nil
	}

	qInsert := "insert into members(id, block_id, login, finished, is_acted) values " + strings.Join(values, ",") + " on conflict do nothing"

	_, execErr := tx.Exec(qInsert)
	if execErr != nil {
		return execErr
	}

	return nil
}

func uniqMembers(in []member) (out []member) {
	mapMembers := make(map[string]interface{})

	for i := range in {
		if _, exists := mapMembers[in[i].WorkID+in[i].StepName+in[i].Login]; !exists {
			out = append(out, in[i])
			mapMembers[in[i].WorkID+in[i].StepName+in[i].Login] = true
		}
	}

	return out
}

func downMembers(_ *sql.Tx) error {
	return nil
}
