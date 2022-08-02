package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputApprover = "approver"
	keyOutputDecision = "decision"
	keyOutputComment  = "comment"
)

type ApproverDecision string

func (a ApproverDecision) String() string {
	return string(a)
}

const (
	ApproverDecisionApproved ApproverDecision = "approved"
	ApproverDecisionRejected ApproverDecision = "rejected"
)

func decisionFromAutoAction(action script.AutoAction) ApproverDecision {
	if action == script.AutoActionApprove {
		return ApproverDecisionApproved
	}
	return ApproverDecisionRejected
}

type Approver struct {
	Decision *ApproverDecision `json:"decision,omitempty"`
	Comment  *string           `json:"comment,omitempty"`
}

type EditingApp struct {
	Approver    string    `json:"approver"`
	Comment     string    `json:"comment"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

type ApproverData struct {
	Type           script.ApproverType `json:"type"`
	Approvers      map[string]struct{} `json:"approvers"`
	Decision       *ApproverDecision   `json:"decision,omitempty"`
	Comment        *string             `json:"comment,omitempty"`
	ActualApprover *string             `json:"actual_approver,omitempty"`

	SLA        int                `json:"sla"`
	AutoAction *script.AutoAction `json:"auto_action,omitempty"`

	DidSLANotification bool `json:"did_sla_notification"`

	LeftToNotify map[string]struct{} `json:"left_to_notify"`

	IsEditable         bool         `json:"is_editable"`
	RepeatPrevDecision bool         `json:"repeat_prev_decision"`
	EditingApp         *EditingApp  `json:"editing_app,omitempty"`
	EditingAppLog      []EditingApp `json:"editing_app_log,omitempty"`
}

func (a *ApproverData) GetDecision() *ApproverDecision {
	return a.Decision
}

func (a *ApproverData) GetRepeatPrevDecision() bool {
	return a.RepeatPrevDecision
}

func (a *ApproverData) GetIsEditable() bool {
	return a.IsEditable
}

func (a *ApproverData) SetDecision(login string, decision ApproverDecision, comment string) error {
	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if decision != ApproverDecisionApproved && decision != ApproverDecisionRejected {
		return fmt.Errorf("unknown decision %s", decision.String())
	}

	a.Decision = &decision
	a.Comment = &comment
	a.ActualApprover = &login

	return nil
}

func (a *ApproverData) SetEditApp(login, comment string, attachments []string) error {
	if a.Decision != nil {
		return errors.New("decision already set")
	}

	editing := &EditingApp{
		Approver:    login,
		Comment:     comment,
		Attachments: attachments,
		CreatedAt:   time.Now(),
	}

	a.EditingAppLog = append(a.EditingAppLog, *editing)

	a.EditingApp = editing

	return nil
}

type updateEditingParams struct {
	Comment     string   `json:"comment"`
	Attachments []string `json:"attachments"`
}

type ApproverUpdateParams struct {
	Decision ApproverDecision `json:"decision"`
	Comment  string           `json:"comment"`
}

func (a *ApproverUpdateParams) Validate() error {
	if a.Decision != ApproverDecisionApproved && a.Decision != ApproverDecisionRejected {
		return errors.New("unknown decision")
	}

	return nil
}

type ApproverResult struct {
	Login    string           `json:"login"`
	Decision ApproverDecision `json:"decision"`
	Comment  string           `json:"comment,omitempty"`
}

type GoApproverBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string
	State  *ApproverData

	Pipeline *ExecutablePipeline
}

func (gb *GoApproverBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusFinished
		}
		return StatusNoSuccess
	}

	if gb.State.EditingApp != nil {
		return StatusIdle
	}

	return StatusRunning
}

func (gb *GoApproverBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.EditingApp != nil {
		return StatusWait
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusApproved
		}
		return StatusApprovementRejected
	}

	return StatusApprovement
}

func (gb *GoApproverBlock) GetType() string {
	return BlockGoApproverID
}

func (gb *GoApproverBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoApproverBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoApproverBlock) IsScenario() bool {
	return false
}

const (
	AutoActionComment = "Выполнено автоматическое действие по истечению SLA"
	AutoApprover      = "auto_approve"
)

// nolint:dupl // other block
func (gb *GoApproverBlock) dumpCurrState(ctx c.Context, id uuid.UUID) error {
	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	content, err := json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	return gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		Status:      string(StatusFinished),
	})
}

//nolint:dupl // maybe later
func (gb *GoApproverBlock) handleNotifications(ctx c.Context, id uuid.UUID, stepCtx *stepCtx) (bool, error) {
	if len(gb.State.LeftToNotify) == 0 {
		return false, nil
	}
	l := logger.GetLogger(ctx)

	emails := make([]string, 0, len(gb.State.Approvers))
	for approver := range gb.State.Approvers {
		email, err := gb.Pipeline.People.GetUserEmail(ctx, approver)
		if err != nil {
			l.WithError(err).Error("couldn't get email")
		}
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return false, nil
	}
	err := gb.Pipeline.Sender.SendNotification(ctx, emails,
		mail.NewApplicationPersonStatusNotification(
			stepCtx.workNumber,
			stepCtx.workTitle,
			statusToTaskAction[StatusApprovement],
			ComputeDeadline(stepCtx.stepStart, gb.State.SLA),
			gb.Pipeline.currDescription,
			gb.Pipeline.Sender.SdAddress))
	if err != nil {
		return false, err
	}

	left := gb.State.LeftToNotify
	gb.State.LeftToNotify = map[string]struct{}{}

	if err := gb.dumpCurrState(ctx, id); err != nil {
		gb.State.LeftToNotify = left
		return false, err
	}
	return true, nil
}

func (gb *GoApproverBlock) handleSLA(ctx c.Context, id uuid.UUID, stepCtx *stepCtx) (bool, error) {
	if gb.State.DidSLANotification {
		return false, nil
	}
	if CheckBreachSLA(stepCtx.stepStart, time.Now(), gb.State.SLA) {
		l := logger.GetLogger(ctx)

		// nolint:dupl // handle approvers
		if gb.State.SLA > 8 {
			emails := make([]string, 0, len(gb.State.Approvers))
			for approver := range gb.State.Approvers {
				email, err := gb.Pipeline.People.GetUserEmail(ctx, approver)
				if err != nil {
					l.WithError(err).Error("couldn't get email")
				}
				emails = append(emails, email)
			}
			if len(emails) == 0 {
				return false, nil
			}
			err := gb.Pipeline.Sender.SendNotification(ctx, emails,
				mail.NewApprovementSLATemplate(stepCtx.workNumber, stepCtx.workTitle, gb.Pipeline.Sender.SdAddress))
			if err != nil {
				return false, err
			}
		}

		gb.State.DidSLANotification = true

		if gb.State.AutoAction != nil {
			if err := gb.setApproverDecision(ctx,
				id,
				AutoApprover,
				ApproverUpdateParams{
					Decision: decisionFromAutoAction(*gb.State.AutoAction),
					Comment:  AutoActionComment,
				}); err != nil {
				gb.State.DidSLANotification = false
				return false, err
			}
		} else {
			if err := gb.dumpCurrState(ctx, id); err != nil {
				gb.State.DidSLANotification = false
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

//nolint:gocyclo //ok
func (gb *GoApproverBlock) DebugRun(ctx c.Context, stepCtx *stepCtx, runCtx *store.VariableStore) (err error) {
	ctx, s := trace.StartSpan(ctx, "run_go_approver_block")
	defer s.End()

	// TODO: fix
	// runCtx.AddStep(gb.Name)

	l := logger.GetLogger(ctx)

	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	// check state from database
	var step *entity.Step
	step, err = gb.Pipeline.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	} else if step == nil {
		l.Error(err)
		return nil
	}

	// get state from step.State
	data, ok := step.State[gb.Name]
	if !ok {
		return nil //TODO: log error?
	}

	var state ApproverData
	err = json.Unmarshal(data, &state)
	if err != nil {
		return errors.Wrap(err, "invalid format of go-approver-block state")
	}

	if state.Type == script.ApproverTypeFromSchema {
		// get approver from application body
		var allVariables map[string]interface{}
		allVariables, err = runCtx.GrabStorage()
		if err != nil {
			return errors.Wrap(err, "Unable to grab variables storage")
		}

		for approverVariableRef := range state.Approvers {
			approverVar := getVariable(allVariables, approverVariableRef)

			if approverVar == nil {
				return errors.Wrap(err, "Unable to find approver by variable reference")
			}

			if actualApproverUsername, castOK := approverVar.(string); castOK {
				state.Approvers[actualApproverUsername] = state.Approvers[approverVariableRef]
				delete(state.Approvers, approverVariableRef)
			}
		}
	}

	gb.State = &state

	if step.Status != string(StatusIdle) {
		handled, errSLA := gb.handleSLA(ctx, id, stepCtx)
		if errSLA != nil {
			l.WithError(errSLA).Error("couldn't handle sla")
		}

		if handled {
			// go for another loop cause we may have updated the state at db
			return gb.DebugRun(ctx, stepCtx, runCtx)
		}

		handled, err = gb.handleNotifications(ctx, id, stepCtx)
		if err != nil {
			l.WithError(err).Error("couldn't handle notifications")
		}
		if handled {
			// go for another loop cause we may have updated the state at db
			return gb.DebugRun(ctx, stepCtx, runCtx)
		}
	}

	// check decision
	decision := gb.State.GetDecision()

	if decision == nil && len(gb.State.EditingAppLog) == 0 && gb.State.GetIsEditable() {
		gb.setEditingAppLogFromPreviousBlock(ctx, &setEditingAppLogDTO{
			id:       id,
			runCtx:   runCtx,
			workID:   gb.Pipeline.TaskID,
			stepName: step.Name,
		})
	}

	if decision == nil && gb.State.GetRepeatPrevDecision() {
		if gb.trySetPreviousDecision(ctx, &getPreviousDecisionDTO{
			id:       id,
			runCtx:   runCtx,
			workID:   gb.Pipeline.TaskID,
			stepName: step.Name,
		}) {
			return nil
		}
	}

	// nolint:dupl // not dupl?
	if decision != nil {
		var actualApprover, comment string

		if state.ActualApprover != nil {
			actualApprover = *state.ActualApprover
		}

		if state.Comment != nil {
			comment = *state.Comment
		}

		runCtx.SetValue(gb.Output[keyOutputApprover], actualApprover)
		runCtx.SetValue(gb.Output[keyOutputDecision], decision.String())
		runCtx.SetValue(gb.Output[keyOutputComment], comment)

		var stateBytes []byte
		stateBytes, err = json.Marshal(gb.State)
		if err != nil {
			return err
		}

		runCtx.ReplaceState(gb.Name, stateBytes)
	}

	return nil
}

type getPreviousDecisionDTO struct {
	id       uuid.UUID
	runCtx   *store.VariableStore
	workID   uuid.UUID
	stepName string
}

func (gb *GoApproverBlock) trySetPreviousDecision(ctx c.Context, dto *getPreviousDecisionDTO) (isPrevDecisionAssigned bool) {
	l := logger.GetLogger(ctx)

	var step *entity.Step
	var parentStep *entity.Step
	var err error

	step, err = gb.Pipeline.Storage.GetTaskStepById(ctx, dto.id)
	if err != nil {
		l.Error(err)
		return
	}

	parentStep, err = gb.Pipeline.Storage.GetParentTaskStepByName(ctx, dto.workID, dto.stepName)
	if err != nil {
		l.Error(err)
		return false
	} else if parentStep == nil {
		l.Error("trySetPreviousDecision: step is nil")
		return false
	}

	// get state from step.State
	data, ok := parentStep.State[dto.stepName]
	if !ok {
		l.Error("trySetPreviousDecision: step state is not found: " + dto.stepName)
		return false
	}

	var parentState ApproverData
	err = json.Unmarshal(data, &parentState)
	if err != nil {
		l.Error("trySetPreviousDecision: invalid format of go-approver-block state")
		return false
	}

	if parentState.Decision != nil {
		var actualApprover, comment string

		if parentState.ActualApprover != nil {
			actualApprover = *parentState.ActualApprover
		}

		if parentState.Comment != nil {
			comment = *parentState.Comment
		}

		dto.runCtx.SetValue(gb.Output[keyOutputApprover], actualApprover)
		dto.runCtx.SetValue(gb.Output[keyOutputDecision], parentState.Decision.String())
		dto.runCtx.SetValue(gb.Output[keyOutputComment], comment)

		gb.State.ActualApprover = &actualApprover
		gb.State.Comment = &comment
		gb.State.Decision = parentState.Decision

		var stateBytes []byte
		stateBytes, err = json.Marshal(gb.State)
		if err != nil {
			l.Error("trySetPreviousDecision: ", err)
			return false
		}

		step.State[gb.Name], err = json.Marshal(store.NewFromStep(step))
		if err != nil {
			l.Error("trySetPreviousDecision: ", err)
			return
		}

		err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
			Id:          dto.id,
			Content:     stateBytes,
			BreakPoints: parentStep.BreakPoints,
			Status:      string(StatusRunning),
		})
		if err != nil {
			l.Error("trySetPreviousDecision.UpdateStepContext: ", err)
			return
		}

		dto.runCtx.ReplaceState(gb.Name, stateBytes)
	}

	return true
}

func (gb *GoApproverBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := rejectedSocket
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ApproverDecisionApproved {
		key = approvedSocket
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		key = editAppSocket
	}

	nexts, ok := gb.Nexts[key]
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoApproverBlock) Skipped(_ *store.VariableStore) []string {
	key := approvedSocket
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ApproverDecisionApproved {
		key = rejectedSocket
	}
	return gb.Nexts[key]
}

func (gb *GoApproverBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoApproverBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoApproverID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputApprover,
				Type:    "string",
				Comment: "approver login which made a decision",
			},
			{
				Name:    keyOutputDecision,
				Type:    "string",
				Comment: "block decision",
			},
			{
				Name:    keyOutputComment,
				Type:    "string",
				Comment: "approver comment",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoApproverID,
			Params: &script.ApproverParams{
				Approver:           "",
				Type:               "",
				SLA:                0,
				IsEditable:         false,
				RepeatPrevDecision: false,
			},
		},
		Sockets: []string{approvedSocket, rejectedSocket, editAppSocket},
	}
}

// nolint:dupl // another block
func createGoApproverBlock(name string, ef *entity.EriusFunc, pipeline *ExecutablePipeline) (*GoApproverBlock, error) {
	b := &GoApproverBlock{
		Name:   name,
		Title:  ef.Title,
		Input:  map[string]string{},
		Output: map[string]string{},
		Nexts:  ef.Next,

		Pipeline: pipeline,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	// TODO: check existence of keyApproverDecision in Output

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.ApproverParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get approver parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid approver parameters")
	}

	// TODO add support for group

	b.State = &ApproverData{
		Type: params.Type,
		Approvers: map[string]struct{}{
			params.Approver: {},
		},
		SLA:        params.SLA,
		AutoAction: params.AutoAction,
		LeftToNotify: map[string]struct{}{
			params.Approver: {},
		},
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
	}

	return b, nil
}
