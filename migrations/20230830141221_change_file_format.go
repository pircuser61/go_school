package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func init() {
	goose.AddMigration(upChangeFileFormat, downChangeFileFormat)
}

type approverState struct {
	Type                json.RawMessage     `json:"type"`
	Approvers           json.RawMessage     `json:"approvers"`
	Decision            json.RawMessage     `json:"decision,omitempty"`
	DecisionAttachments []entity.Attachment `json:"decision_attachments,omitempty"`
	Comment             json.RawMessage     `json:"comment,omitempty"`
	ActualApprover      json.RawMessage     `json:"actual_approver,omitempty"`
	ApprovementRule     json.RawMessage     `json:"approvementRule,omitempty"`
	ApproverLog         ArrApproverLogEntry `json:"approver_log,omitempty"`

	IsEditable         json.RawMessage       `json:"is_editable"`
	RepeatPrevDecision json.RawMessage       `json:"repeat_prev_decision"`
	EditingApp         *ApproverEditingApp   `json:"editing_app,omitempty"`
	EditingAppLog      ArrApproverEditingApp `json:"editing_app_log,omitempty"`

	FormsAccessibility json.RawMessage `json:"forms_accessibility,omitempty"`

	ApproversGroupID   json.RawMessage `json:"approvers_group_id"`
	ApproversGroupName json.RawMessage `json:"approvers_group_name"`

	ApproversGroupIDPath json.RawMessage `json:"approvers_group_id_path,omitempty"`

	AddInfo ArrAdditionalInfo `json:"additional_info,omitempty"`

	ApproveStatusName string `json:"approve_status_name"`

	SLA                          int    `json:"sla"`
	CheckSLA                     bool   `json:"check_sla"`
	SLAChecked                   bool   `json:"sla_checked"`
	HalfSLAChecked               bool   `json:"half_sla_checked"`
	ReworkSLA                    int    `json:"rework_sla"`
	CheckReworkSLA               bool   `json:"check_rework_sla"`
	CheckDayBeforeSLARequestInfo bool   `json:"check_day_before_sla_request_info"`
	WorkType                     string `json:"work_type"`

	AutoAction json.RawMessage `json:"auto_action,omitempty"`

	ActionList json.RawMessage `json:"action_list"`

	AdditionalApprovers ArrAdditionalApprover `json:"additional_approvers"`
}

type ApproverLogEntry struct {
	Login          json.RawMessage     `json:"login"`
	Decision       json.RawMessage     `json:"decision"`
	Comment        json.RawMessage     `json:"comment"`
	CreatedAt      json.RawMessage     `json:"created_at"`
	Attachments    []entity.Attachment `json:"attachments"`
	AddedApprovers json.RawMessage     `json:"added_approvers"`
	LogType        json.RawMessage     `json:"log_type"`
	DelegateFor    json.RawMessage     `json:"delegate_for"`
}

type ArrApproverLogEntry []ApproverLogEntry

func (at *ArrApproverLogEntry) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []ApproverLogEntry
		atTemp  ApproverLogEntry
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

func (at *ApproverLogEntry) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Login          json.RawMessage     `json:"login"`
		Decision       json.RawMessage     `json:"decision"`
		Comment        json.RawMessage     `json:"comment"`
		CreatedAt      json.RawMessage     `json:"created_at"`
		Attachments    []entity.Attachment `json:"attachments"`
		AddedApprovers json.RawMessage     `json:"added_approvers"`
		LogType        json.RawMessage     `json:"log_type"`
		DelegateFor    json.RawMessage     `json:"delegate_for"`
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

type ApproverEditingApp struct {
	Approver    json.RawMessage     `json:"approver"`
	Comment     json.RawMessage     `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	CreatedAt   json.RawMessage     `json:"created_at"`
	DelegateFor json.RawMessage     `json:"delegate_for"`
}

type ArrApproverEditingApp []ApproverEditingApp

func (at *ArrApproverEditingApp) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []ApproverEditingApp
		atTemp  ApproverEditingApp
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

func (at *ApproverEditingApp) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Approver    json.RawMessage     `json:"approver"`
		Comment     json.RawMessage     `json:"comment"`
		Attachments []entity.Attachment `json:"attachments"`
		CreatedAt   json.RawMessage     `json:"created_at"`
		DelegateFor json.RawMessage     `json:"delegate_for"`
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

type AdditionalInfo struct {
	ID          json.RawMessage     `json:"id"`
	Login       json.RawMessage     `json:"login"`
	Comment     json.RawMessage     `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	LinkID      json.RawMessage     `json:"link_id,omitempty"`
	Type        json.RawMessage     `json:"type"`
	CreatedAt   json.RawMessage     `json:"created_at"`
	DelegateFor json.RawMessage     `json:"delegate_for"`
}

type ArrAdditionalInfo []AdditionalInfo

func (at *ArrAdditionalInfo) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []AdditionalInfo
		atTemp  AdditionalInfo
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

func (at *AdditionalInfo) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		ID          json.RawMessage     `json:"id"`
		Login       json.RawMessage     `json:"login"`
		Comment     json.RawMessage     `json:"comment"`
		Attachments []entity.Attachment `json:"attachments"`
		LinkID      json.RawMessage     `json:"link_id,omitempty"`
		Type        json.RawMessage     `json:"type"`
		CreatedAt   json.RawMessage     `json:"created_at"`
		DelegateFor json.RawMessage     `json:"delegate_for"`
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

type AdditionalApprover struct {
	ApproverLogin     json.RawMessage     `json:"approver_login"`
	BaseApproverLogin json.RawMessage     `json:"base_approver_login"`
	Question          json.RawMessage     `json:"question"`
	Comment           json.RawMessage     `json:"comment"`
	Attachments       []entity.Attachment `json:"attachments"`
	Decision          json.RawMessage     `json:"decision"`
	CreatedAt         json.RawMessage     `json:"created_at"`
	DecisionTime      json.RawMessage     `json:"decision_time"`
}

type ArrAdditionalApprover []AdditionalApprover

func (at *ArrAdditionalApprover) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []AdditionalApprover
		atTemp  AdditionalApprover
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

func (at *AdditionalApprover) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		ApproverLogin     json.RawMessage     `json:"approver_login"`
		BaseApproverLogin json.RawMessage     `json:"base_approver_login"`
		Question          json.RawMessage     `json:"question"`
		Comment           json.RawMessage     `json:"comment"`
		Attachments       []entity.Attachment `json:"attachments"`
		Decision          json.RawMessage     `json:"decision"`
		CreatedAt         json.RawMessage     `json:"created_at"`
		DecisionTime      json.RawMessage     `json:"decision_time"`
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

type ExecutionData struct {
	ExecutionType       json.RawMessage     `json:"execution_type"`
	Executors           json.RawMessage     `json:"executors"`
	Decision            json.RawMessage     `json:"decision,omitempty"`
	DecisionAttachments []entity.Attachment `json:"decision_attachments,omitempty"`
	DecisionComment     json.RawMessage     `json:"comment,omitempty"`
	ActualExecutor      json.RawMessage     `json:"actual_executor,omitempty"`
	DelegateFor         json.RawMessage     `json:"delegate_for"`

	EditingApp               *ExecutorEditApp           `json:"editing_app,omitempty"`
	EditingAppLog            ArrExecutorEditApp         `json:"editing_app_log,omitempty"`
	ChangedExecutorsLogs     ArrChangeExecutorLog       `json:"change_executors_logs,omitempty"`
	RequestExecutionInfoLogs ArrRequestExecutionInfoLog `json:"request_execution_info_logs,omitempty"`
	FormsAccessibility       json.RawMessage            `json:"forms_accessibility,omitempty"`

	ExecutorsGroupID   json.RawMessage `json:"executors_group_id"`
	ExecutorsGroupName json.RawMessage `json:"executors_group_name"`

	ExecutorsGroupIDPath json.RawMessage `json:"executors_group_id_path,omitempty"`

	IsTakenInWork               json.RawMessage `json:"is_taken_in_work"`
	IsExecutorVariablesResolved json.RawMessage `json:"is_executor_variables_resolved"`

	IsEditable         json.RawMessage `json:"is_editable"`
	RepeatPrevDecision json.RawMessage `json:"repeat_prev_decision"`
	UseActualExecutor  json.RawMessage `json:"use_actual_executor"`

	SLA                          json.RawMessage `json:"sla"`
	CheckSLA                     json.RawMessage `json:"check_sla"`
	SLAChecked                   json.RawMessage `json:"sla_checked"`
	HalfSLAChecked               json.RawMessage `json:"half_sla_checked"`
	ReworkSLA                    json.RawMessage `json:"rework_sla"`
	CheckReworkSLA               json.RawMessage `json:"check_rework_sla"`
	CheckDayBeforeSLARequestInfo json.RawMessage `json:"check_day_before_sla_request_info"`
	WorkType                     json.RawMessage `json:"work_type"`
}

type ExecutorEditApp struct {
	Executor    json.RawMessage     `json:"executor"`
	Comment     json.RawMessage     `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	CreatedAt   json.RawMessage     `json:"created_at"`
	DelegateFor json.RawMessage     `json:"delegate_for"`
}

type ArrExecutorEditApp []ExecutorEditApp

func (at *ArrExecutorEditApp) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []ExecutorEditApp
		atTemp  ExecutorEditApp
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

func (at *ExecutorEditApp) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Executor    json.RawMessage     `json:"executor"`
		Comment     json.RawMessage     `json:"comment"`
		Attachments []entity.Attachment `json:"attachments"`
		CreatedAt   json.RawMessage     `json:"created_at"`
		DelegateFor json.RawMessage     `json:"delegate_for"`
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

type ChangeExecutorLog struct {
	OldLogin    json.RawMessage     `json:"old_login"`
	NewLogin    json.RawMessage     `json:"new_login"`
	Comment     json.RawMessage     `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	CreatedAt   json.RawMessage     `json:"created_at"`
}

type ArrChangeExecutorLog []ChangeExecutorLog

func (at *ArrChangeExecutorLog) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []ChangeExecutorLog
		atTemp  ChangeExecutorLog
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

func (at *ChangeExecutorLog) UnmarshalJSON(b []byte) error {
	var (
		atTemp struct {
			OldLogin    json.RawMessage     `json:"old_login"`
			NewLogin    json.RawMessage     `json:"new_login"`
			Comment     json.RawMessage     `json:"comment"`
			Attachments []entity.Attachment `json:"attachments"`
			CreatedAt   json.RawMessage     `json:"created_at"`
		}
		stTemp string
	)

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

type RequestExecutionInfoLog struct {
	Login       json.RawMessage     `json:"login"`
	Comment     json.RawMessage     `json:"comment"`
	CreatedAt   json.RawMessage     `json:"created_at"`
	ReqType     json.RawMessage     `json:"req_type"`
	Attachments []entity.Attachment `json:"attachments"`
	DelegateFor json.RawMessage     `json:"delegate_for"`
}

type ArrRequestExecutionInfoLog []RequestExecutionInfoLog

func (at *ArrRequestExecutionInfoLog) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []RequestExecutionInfoLog
		atTemp  RequestExecutionInfoLog
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

func (at *RequestExecutionInfoLog) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Login       json.RawMessage     `json:"login"`
		Comment     json.RawMessage     `json:"comment"`
		CreatedAt   json.RawMessage     `json:"created_at"`
		ReqType     json.RawMessage     `json:"req_type"`
		Attachments []entity.Attachment `json:"attachments"`
		DelegateFor json.RawMessage     `json:"delegate_for"`
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

type SignData struct {
	Type             json.RawMessage     `json:"type"`
	Signers          json.RawMessage     `json:"signers"`
	SignatureType    json.RawMessage     `json:"signature_type"`
	Decision         json.RawMessage     `json:"decision,omitempty"`
	Comment          json.RawMessage     `json:"comment,omitempty"`
	ActualSigner     json.RawMessage     `json:"actual_signer,omitempty"`
	Attachments      []entity.Attachment `json:"attachments,omitempty"`
	SigningRule      json.RawMessage     `json:"signing_rule,omitempty"`
	SignatureCarrier json.RawMessage     `json:"signature_carrier,omitempty"`
	SignLog          ArrSignLogEntry     `json:"sign_log,omitempty"`

	FormsAccessibility json.RawMessage `json:"forms_accessibility,omitempty"`

	SignerGroupID   json.RawMessage `json:"signer_group_id,omitempty"`
	SignerGroupName json.RawMessage `json:"signer_group_name,omitempty"`

	SLA        json.RawMessage `json:"sla,omitempty"`
	CheckSLA   json.RawMessage `json:"check_sla,omitempty"`
	SLAChecked json.RawMessage `json:"sla_checked"`
	AutoReject json.RawMessage `json:"auto_reject,omitempty"`
	WorkType   json.RawMessage `json:"work_type,omitempty"`
}

type SignLogEntry struct {
	Login       json.RawMessage     `json:"login"`
	Decision    json.RawMessage     `json:"decision"`
	Comment     json.RawMessage     `json:"comment"`
	CreatedAt   json.RawMessage     `json:"created_at"`
	Attachments []entity.Attachment `json:"attachments,omitempty"`
}

type ArrSignLogEntry []SignLogEntry

func (at *ArrSignLogEntry) UnmarshalJSON(b []byte) error {
	var (
		arrTemp []SignLogEntry
		atTemp  SignLogEntry
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

func (at *SignLogEntry) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		Login       json.RawMessage     `json:"login"`
		Decision    json.RawMessage     `json:"decision"`
		Comment     json.RawMessage     `json:"comment"`
		CreatedAt   json.RawMessage     `json:"created_at"`
		Attachments []entity.Attachment `json:"attachments,omitempty"`
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

		var (
			state string
			ID    string
		)

		scanErr := rows.Scan(
			&ID,
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
				data = &approverState{}
			case strings.Contains(key, "execution"):
				data = &ExecutionData{}
			case strings.Contains(key, "sign"):
				data = &SignData{}
			}

			if data != nil {
				err := json.Unmarshal(val, &data)
				if err != nil {
					fmt.Println(ID)

					return err
				}

				resJSON, mErr := json.Marshal(data)
				if mErr != nil {
					return mErr
				}

				resultMap[key] = resJSON
			} else {
				resultMap[key] = val
			}
		}

		result = append(result, resultStruct{
			resultMap: resultMap,
			id:        ID,
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

func downChangeFileFormat(_ *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
