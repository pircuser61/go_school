package pipeline

import (
	c "context"
	"fmt"
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
	formName            = "form_name"
	formFillFormAction  = "fill_form"
	formStartWorkAction = "form_executor_start_work"
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
	IsFilled               bool                    `json:"is_filled"`
	IsTakenInWork          bool                    `json:"is_taken_in_work"`
	IsReentry              bool                    `json:"is_reentry"`
	ActualExecutor         *string                 `json:"actual_executor,omitempty"`
	ChangesLog             []ChangesLogItem        `json:"changes_log"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	IsRevoked bool `json:"is_revoked"`

	SLA            int    `json:"sla"`
	CheckSLA       bool   `json:"check_sla"`
	SLAChecked     bool   `json:"sla_checked"`
	HalfSLAChecked bool   `json:"half_sla_checked"`
	WorkType       string `json:"work_type"`

	HideExecutorFromInitiator bool `json:"hide_executor_from_initiator"`

	Mapping script.JSONSchemaProperties `json:"mapping"`

	IsEditable      *bool                       `json:"is_editable"`
	ReEnterSettings *script.FormReEnterSettings `json:"form_re_enter_settings,omitempty"`
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

	formNames := []string{gb.Name}

	for _, v := range gb.State.FormsAccessibility {
		if _, ok := gb.RunContext.VarStore.State[v.NodeID]; !ok {
			continue
		}

		if gb.Name == v.NodeID {
			continue
		}

		if v.AccessType == "ReadWrite" {
			formNames = append(formNames, v.NodeID)
		}
	}

	actions := []MemberAction{
		{
			ID:   formFillFormAction,
			Type: ActionTypeCustom,
			Params: map[string]interface{}{
				formName: formNames,
			},
		},
	}

	return actions
}

func (gb *GoFormBlock) Deadlines(ctx c.Context) ([]Deadline, error) {
	if gb.State.IsRevoked {
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

	formMapping, err := script.MapData(gb.State.Mapping, script.RestoreMapStructure(variables), []string{})
	if err != nil {
		return err
	}

	gb.State.ApplicationBody = formMapping

	personData := &servicedesc.SsoPerson{
		Username: AutoFillUser,
	}

	gb.State.Executors = map[string]struct{}{
		AutoFillUser: {},
	}
	gb.State.ChangesLog = append([]ChangesLogItem{
		{
			ApplicationBody: formMapping,
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
