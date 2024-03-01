package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputFormExecutor = "executor"
	keyOutputFormBody     = "application_body"
)

const (
	disabled                   = "disabled"
	formName                   = "form_name"
	formFillFormAction         = "fill_form"
	formFillFormDisabledAction = "fill_form_disabled"
	formStartWorkAction        = "form_executor_start_work"
)

const AutoFillUser = "auto_fill"

type ChangesLogItem struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
	CreatedAt       time.Time              `json:"created_at"`
	Executor        string                 `json:"executor,omitempty"`
	DelegateFor     string                 `json:"delegate_for"`
}

type FormData struct {
	FormExecutorType       script.FormExecutorType `json:"form_executor_type"`
	FormGroupID            string                  `json:"form_group_id"`
	FormExecutorsGroupName string                  `json:"form_executors_group_name"`
	FormGroupIDPath        *string                 `json:"form_group_id_path,omitempty"`
	SchemaID               string                  `json:"schema_id"`
	Executors              map[string]struct{}     `json:"executors"`
	InitialExecutors       map[string]struct{}     `json:"initial_executors"`
	Description            string                  `json:"description"`
	ApplicationBody        map[string]interface{}  `json:"application_body"`
	Constants              map[string]interface{}  `json:"constants"`
	IsFilled               bool                    `json:"is_filled"`
	IsTakenInWork          bool                    `json:"is_taken_in_work"`
	IsReentry              bool                    `json:"is_reentry"`
	ActualExecutor         *string                 `json:"actual_executor,omitempty"`
	ChangesLog             []ChangesLogItem        `json:"changes_log"`
	HiddenFields           []string                `json:"hidden_fields"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	IsRevoked bool `json:"is_revoked"`

	SLA            int    `json:"sla"`
	CheckSLA       bool   `json:"check_sla"`
	SLAChecked     bool   `json:"sla_checked"`
	HalfSLAChecked bool   `json:"half_sla_checked"`
	WorkType       string `json:"work_type"`

	HideExecutorFromInitiator bool `json:"hide_executor_from_initiator"`

	Mapping         script.JSONSchemaProperties `json:"mapping"`
	FullFormMapping string                      `json:"full_form_mapping"`

	AttachmentFields []string          `json:"attachment_fields"`
	Keys             map[string]string `json:"keys"`

	CheckRequiredForm bool                        `json:"checkRequiredForm"`
	IsEditable        *bool                       `json:"is_editable"`
	ReEnterSettings   *script.FormReEnterSettings `json:"form_re_enter_settings,omitempty"`
}

type GoFormBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *FormData

	RunContext *BlockRunContext

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent
}

func (gb *GoFormBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoFormBlock) Members() []Member {
	members := []Member{}
	for login := range gb.State.Executors {
		members = append(members, Member{
			Login:                login,
			Actions:              gb.formActions(),
			IsActed:              gb.isFormUserActed(login),
			ExecutionGroupMember: false,
		})
	}

	return members
}

func (gb *GoFormBlock) isFormUserActed(login string) bool {
	for i := range gb.State.ChangesLog {
		if gb.State.ChangesLog[i].Executor == login {
			return true
		}
	}

	return false
}

func (gb *GoFormBlock) formActions() []MemberAction {
	if gb.State.IsFilled {
		return []MemberAction{}
	}

	if !gb.State.IsTakenInWork {
		action := MemberAction{
			ID:   formStartWorkAction,
			Type: ActionTypePrimary,
		}

		return []MemberAction{action}
	}

	formActionNames := gb.CreateFillFormActions()

	disabledForm := make([]string, 0)
	fillForm := make([]string, 0)

	for k, v := range formActionNames {
		if v {
			disabledForm = append(disabledForm, k)

			continue
		}

		fillForm = append(fillForm, k)
	}

	sort.Strings(fillForm)
	sort.Strings(disabledForm)

	actions := []MemberAction{
		{
			ID:   formFillFormDisabledAction,
			Type: ActionTypeCustom,
			Params: map[string]interface{}{
				formName: disabledForm,
				disabled: true,
			},
		},
		{
			ID:   formFillFormAction,
			Type: ActionTypeCustom,
			Params: map[string]interface{}{
				formName: fillForm,
			},
		},
	}

	return actions
}

func (gb *GoFormBlock) CreateFillFormActions() map[string]bool {
	actions := make(map[string]bool, 0)

FormLabel:
	for _, form := range gb.State.FormsAccessibility {
		formState, ok := gb.RunContext.VarStore.State[form.NodeID]
		if !ok {
			continue
		}

		switch form.AccessType {
		case readWriteAccessType:
			actions[form.NodeID] = false
		case requiredFillAccessType:
			var formData FormData
			if err := json.Unmarshal(formState, &formData); err != nil {
				return actions
			}

			users := make(map[string]struct{}, 0)

			for user := range gb.State.Executors {
				users[user] = struct{}{}
			}

			for user := range gb.State.InitialExecutors {
				users[user] = struct{}{}
			}

			if !formData.IsFilled {
				actions[form.NodeID] = false

				continue
			}

			for _, v := range formData.ChangesLog {
				if _, findOk := users[v.Executor]; findOk {
					actions[form.NodeID] = false

					continue FormLabel
				}
			}

			actions[form.NodeID] = true
		}
	}

	return actions
}

func (gb *GoFormBlock) Deadlines(ctx c.Context) ([]Deadline, error) {
	if gb.State.IsRevoked || gb.State.IsFilled {
		return []Deadline{}, nil
	}

	deadlines := make([]Deadline, 0, 2)

	if gb.State.CheckSLA {
		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		})

		if getSLAInfoErr != nil {
			return nil, getSLAInfoErr
		}

		if !gb.State.SLAChecked {
			deadlines = append(deadlines,
				Deadline{
					Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime,
						float32(gb.State.SLA),
						slaInfoPtr),
					Action: entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if !gb.State.HalfSLAChecked && gb.State.SLA >= 8 {
			deadlines = append(deadlines,
				Deadline{
					Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime,
						float32(gb.State.SLA)/2,
						slaInfoPtr),
					Action: entity.TaskUpdateActionHalfSLABreach,
				},
			)
		}
	}

	return deadlines, nil
}

func (gb *GoFormBlock) UpdateManual() bool {
	return true
}

func (gb *GoFormBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsFilled {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoFormBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State != nil && gb.State.IsFilled {
		return StatusDone, "", ""
	}

	if gb.State.IsReentry {
		return StatusWait, fmt.Sprintf("Заявку вернули на доработку: %s", time.Now().Format("02.01.2006 15:04")), ""
	}

	return StatusExecution, "", ""
}

func (gb *GoFormBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoFormBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoFormBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoFormID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputFormExecutor: {
					Type:        "object",
					Description: "person object from sso",
					Format:      "SsoPerson",
					Properties:  people.GetSsoPersonSchemaProperties(),
				},
				keyOutputFormBody: {
					Type:        "object",
					Description: "form body",
				},
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoFormID,
			Params: &script.FormParams{
				FormsAccessibility: []script.FormAccessibility{},
				Mapping:            script.JSONSchemaProperties{},
				FullFormMapping:    "",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *GoFormBlock) handleAutoFillForm() error {
	variables, err := getVariables(gb.RunContext.VarStore)
	if err != nil {
		return err
	}

	switch {
	case gb.State.FullFormMapping != "":
		formMapping, ok := getVariable(variables, gb.State.FullFormMapping).(map[string]interface{})
		if !ok {
			return fmt.Errorf("cannot assert variable to map[string]interface{}")
		}

		validSchema := &script.JSONSchemaPropertiesValue{
			Type:       "object",
			Properties: gb.State.Mapping,
		}

		if err = script.ValidateParam(formMapping, validSchema); err != nil {
			return fmt.Errorf("mapping is not valid: %w", err)
		}

		if gb.State.CheckRequiredForm {
			byteSchema, marshalErr := json.Marshal(validSchema)
			if marshalErr != nil {
				return marshalErr
			}

			byteApplicationBody, marshalApBodyErr := json.Marshal(gb.State.ApplicationBody)
			if marshalApBodyErr != nil {
				return marshalApBodyErr
			}

			if validErr := script.ValidateJSONByJSONSchema(string(byteApplicationBody), string(byteSchema)); validErr != nil {
				return validErr
			}
		}

		gb.State.ApplicationBody = formMapping
	case gb.State.Mapping != nil:
		formMapping, mdErr := script.MapData(gb.State.Mapping, script.RestoreMapStructure(variables), []string{})
		if mdErr != nil {
			return mdErr
		}

		gb.State.ApplicationBody = formMapping
	}

	if constErr := script.FillMapWithConstants(gb.State.Constants, gb.State.ApplicationBody); constErr != nil {
		return constErr
	}

	personData := &servicedesc.SsoPerson{
		Username: AutoFillUser,
	}

	gb.State.Executors = map[string]struct{}{
		AutoFillUser: {},
	}
	gb.State.ChangesLog = append([]ChangesLogItem{
		{
			ApplicationBody: gb.State.ApplicationBody,
			CreatedAt:       time.Now(),
			Executor:        personData.Username,
			DelegateFor:     "",
		},
	}, gb.State.ChangesLog...)

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormExecutor], personData)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormBody], gb.State.ApplicationBody)

	gb.State.ActualExecutor = &personData.Username
	gb.State.IsFilled = true

	return nil
}

func (gb *GoFormBlock) handleNotifications(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	if gb.RunContext.skipNotifications {
		return nil
	}

	executors := getSliceFromMapOfStrings(gb.State.Executors)

	fileNames := make([]string, 0)
	emails := make(map[string]mail.Template, 0)

	if !gb.State.IsTakenInWork {
		fileNames = append(fileNames, vRabotuBtn)
	}

	for _, login := range executors {
		em, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")

			continue
		}

		if !gb.State.IsTakenInWork {
			slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
				TaskCompletionIntervals: []entity.TaskCompletionInterval{{
					StartedAt:  gb.RunContext.CurrBlockStartTime,
					FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
				}},
				WorkType: sla.WorkHourType(gb.State.WorkType),
			})
			if getSLAInfoErr != nil {
				return getSLAInfoErr
			}

			emails[em] = mail.NewFormExecutionNeedTakeInWorkTpl(
				&mail.NewFormExecutionNeedTakeInWorkDto{
					WorkNumber: gb.RunContext.WorkNumber,
					WorkTitle:  gb.RunContext.NotifName,
					SdURL:      gb.RunContext.Services.Sender.SdAddress,
					Mailto:     gb.RunContext.Services.Sender.FetchEmail,
					BlockName:  BlockGoFormID,
					Login:      login,
					Deadline:   gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
				},
				gb.State.IsReentry,
			)
		} else {
			slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
				TaskCompletionIntervals: []entity.TaskCompletionInterval{{
					StartedAt:  gb.RunContext.CurrBlockStartTime,
					FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
				}},
				WorkType: sla.WorkHourType(gb.State.WorkType),
			})
			if getSLAInfoErr != nil {
				return getSLAInfoErr
			}

			emails[em] = mail.NewRequestFormExecutionInfoTpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.NotifName,
				gb.RunContext.Services.Sender.SdAddress,
				gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(gb.RunContext.CurrBlockStartTime, gb.State.SLA,
					slaInfoPtr),
				gb.State.IsReentry)
		}
	}

	if len(emails) == 0 {
		return nil
	}

	for i := range emails {
		item := emails[i]

		iconNames := make([]string, 0, len(fileNames)+1)

		iconNames = append(iconNames, item.Image)
		iconNames = append(iconNames, fileNames...)

		files, iconErr := gb.RunContext.GetIcons(iconNames)
		if iconErr != nil {
			return iconErr
		}

		if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{i}, files,
			emails[i]); sendErr != nil {
			return sendErr
		}
	}

	return nil
}
