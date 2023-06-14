package pipeline

import (
	c "context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	e "gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputFormExecutor = "executor"
	keyOutputFormBody     = "application_body"
)

const formFillFormAction = "fill_form"

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
	FormGroupId            string                  `json:"form_group_id"`
	FormExecutorsGroupName string                  `json:"form_executors_group_name"`
	SchemaId               string                  `json:"schema_id"`
	SchemaName             string                  `json:"schema_name"`
	Executors              map[string]struct{}     `json:"executors"`
	Description            string                  `json:"description"`
	ApplicationBody        map[string]interface{}  `json:"application_body"`
	IsFilled               bool                    `json:"is_filled"`
	IsTakenInWork          bool                    `json:"is_taken_in_work"`
	ActualExecutor         *string                 `json:"actual_executor,omitempty"`
	ChangesLog             []ChangesLogItem        `json:"changes_log"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	SLA            int  `json:"sla"`
	CheckSLA       bool `json:"check_sla"`
	SLAChecked     bool `json:"sla_checked"`
	HalfSLAChecked bool `json:"half_sla_checked"`

	HideExecutorFromInitiator bool `json:"hide_executor_from_initiator"`

	Mapping script.JSONSchemaProperties `json:"mapping"`
}

type GoFormBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *FormData

	RunContext *BlockRunContext
}

func (gb *GoFormBlock) Members() []Member {
	members := []Member{}
	for login := range gb.State.Executors {
		members = append(members, Member{
			Login:      login,
			IsFinished: gb.isFormFinished(),
			Actions:    gb.formActions(),
		})
	}

	return members
}

func (gb *GoFormBlock) isFormFinished() bool {
	return gb.State.IsFilled
}

func (gb *GoFormBlock) formActions() []MemberAction {
	if gb.State.IsFilled {
		return []MemberAction{}
	}
	action := MemberAction{
		Id:   formFillFormAction,
		Type: ActionTypeCustom,
	}
	return []MemberAction{action}
}

func (gb *GoFormBlock) Deadlines() []Deadline {
	deadlines := make([]Deadline, 0, 2)

	if gb.State.CheckSLA {
		if !gb.State.SLAChecked {
			deadlines = append(deadlines,
				Deadline{Deadline: ComputeMaxDate(gb.RunContext.currBlockStartTime, float32(gb.State.SLA)),
					Action: entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if !gb.State.HalfSLAChecked && gb.State.SLA >= 8 {
			deadlines = append(deadlines,
				Deadline{Deadline: ComputeMaxDate(gb.RunContext.currBlockStartTime, float32(gb.State.SLA)/2),
					Action: entity.TaskUpdateActionHalfSLABreach,
				},
			)
		}
	}

	return deadlines
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

func (gb *GoFormBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsFilled {
		return StatusDone
	}

	return StatusExecution
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
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputFormExecutor,
				Type:    "SsoPerson",
				Comment: "form executor login",
			},
			{
				Name:    keyOutputFormBody,
				Type:    "object",
				Comment: "form body",
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

// nolint:dupl // another block
func createGoFormBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoFormBlock, error) {
	b := &GoFormBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	rawState, ok := runCtx.VarStore.State[name]
	if ok {
		if err := b.loadState(rawState); err != nil {
			return nil, err
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, nil
}

func (gb *GoFormBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl //different logic
func (gb *GoFormBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
	var params script.FormParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get form parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid form parameters")
	}

	gb.State = &FormData{
		Executors: map[string]struct{}{
			params.Executor: {},
		},
		SchemaId:                  params.SchemaId,
		SLA:                       params.SLA,
		CheckSLA:                  params.CheckSLA,
		SchemaName:                params.SchemaName,
		ChangesLog:                make([]ChangesLogItem, 0),
		FormExecutorType:          params.FormExecutorType,
		ApplicationBody:           map[string]interface{}{},
		FormsAccessibility:        params.FormsAccessibility,
		Mapping:                   params.Mapping,
		HideExecutorFromInitiator: params.HideExecutorFromInitiator,
	}

	switch gb.State.FormExecutorType {
	case script.FormExecutorTypeUser:
		gb.State.Executors = map[string]struct{}{
			params.Executor: {},
		}
	case script.FormExecutorTypeInitiator:
		gb.State.Executors = map[string]struct{}{
			gb.RunContext.Initiator: {},
		}
	case script.FormExecutorTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return err
		}

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				params.Executor: {},
			},
		)
		if resolveErr != nil {
			return err
		}

		gb.State.Executors = resolvedEntities
	case script.FormExecutorTypeAutoFillUser:
		if err = gb.handleAutoFillForm(); err != nil {
			return err
		}
	case script.FormExecutorTypeGroup:
		workGroup, errGroup := gb.RunContext.ServiceDesc.GetWorkGroup(ctx, params.FormGroupId)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get form group with id: "+params.FormGroupId)
		}

		if len(workGroup.People) == 0 {
			//nolint:goimports // bugged golint
			return errors.New("zero form executors in group: " + params.FormGroupId)
		}

		gb.State.Executors = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Executors[workGroup.People[i].Login] = struct{}{}
		}
		gb.State.FormGroupId = params.FormGroupId
		gb.State.FormExecutorsGroupName = workGroup.GroupName
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoFormBlock) handleAutoFillForm() error {
	variables, err := getVariables(gb.RunContext.VarStore)
	if err != nil {
		return err
	}

	formMapping := make(map[string]interface{})

	for k := range gb.State.Mapping {
		varPath := gb.State.Mapping[k]

		variableValue := getVariable(variables, varPath.Value)

		formMapping[k] = variableValue
	}

	gb.State.ApplicationBody = formMapping

	personData := &servicedesc.SsoPerson{
		Username: AutoFillUser,
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
	isGroupExecutors := gb.State.FormExecutorType == script.FormExecutorTypeGroup
	var emailAttachment []e.Attachment

	var emails = make(map[string]mail.Template, 0)
	for _, login := range executors {
		em, getUserEmailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")
			continue
		}

		if isGroupExecutors {
			emails[em] = mail.NewFormExecutionNeedTakeInWorkTpl(gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
				ComputeDeadline(time.Now(), gb.State.SLA),
			)
		} else {
			emails[em] = mail.NewRequestFormExecutionInfoTpl(&mail.NewRequestFormExecutionInfoDto{
				WorkNumber: gb.RunContext.WorkNumber,
				Name:       gb.RunContext.WorkTitle,
				SdUrl:      gb.RunContext.Sender.SdAddress,
				Mailto:     gb.RunContext.Sender.FetchEmail,
				BlockName:  BlockGoFormID,
				Login:      login,
			})
		}
	}

	if len(emails) == 0 {
		return nil
	}

	for i := range emails {
		if sendErr := gb.RunContext.Sender.SendNotification(ctx, []string{i}, emailAttachment, emails[i]); sendErr != nil {
			return sendErr
		}
	}
	return nil
}
