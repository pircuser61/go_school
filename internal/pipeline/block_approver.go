package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"
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
	AutoActionComment = "Выполнено автоматическое действие по истечению SLA"
	AutoApprover      = "auto_approve"

	keyOutputApprover = "approver"
	keyOutputDecision = "decision"
	keyOutputComment  = "comment"
)

type GoApproverBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ApproverData

	Pipeline *ExecutablePipeline
}

func (gb *GoApproverBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusCancel
	}
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusFinished
		}

		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusNoSuccess
		}
	}

	if gb.State.EditingApp != nil {
		return StatusIdle
	}

	if len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIdAddInfo() {
			return StatusIdle
		}
	}

	return StatusRunning
}

func (gb *GoApproverBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusRevoke
	}
	if gb.State != nil && gb.State.EditingApp != nil {
		return StatusWait
	}

	if gb.State != nil && len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIdAddInfo() {
			return StatusWait
		}
		return StatusApprovement
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusApproved
		}
		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusApprovementRejected
		}
	}

	var lastIdx = len(gb.State.RequestApproverInfoLog) - 1
	if len(gb.State.RequestApproverInfoLog) > 0 && gb.State.RequestApproverInfoLog[lastIdx].Type == RequestAddInfoType {
		return StatusWait
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
	descr := gb.Pipeline.currDescription
	additionalDescriptions, err := gb.Pipeline.Storage.GetAdditionalForms(gb.Pipeline.WorkNumber, gb.Name)
	if err != nil {
		return false, err
	}
	for _, item := range additionalDescriptions {
		if item == "" {
			continue
		}
		descr = fmt.Sprintf("%s\n\n%s", descr, item)
	}
	err = gb.Pipeline.Sender.SendNotification(ctx, emails, nil,
		mail.NewApplicationPersonStatusNotification(
			stepCtx.workNumber,
			stepCtx.workTitle,
			statusToTaskAction[StatusApprovement],
			ComputeDeadline(stepCtx.stepStart, gb.State.SLA),
			descr,
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
	const workHoursDay = 8

	if gb.State.DidSLANotification {
		return false, nil
	}
	if CheckBreachSLA(stepCtx.stepStart, time.Now(), gb.State.SLA) {
		l := logger.GetLogger(ctx)

		// nolint:dupl // handle approvers
		if gb.State.SLA > workHoursDay {
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

			tpl := mail.NewApprovementSLATemplate(stepCtx.workNumber, stepCtx.workTitle, gb.Pipeline.Sender.SdAddress)
			err := gb.Pipeline.Sender.SendNotification(ctx, emails, nil, tpl)
			if err != nil {
				return false, err
			}
		}

		gb.State.DidSLANotification = true

		if gb.State.AutoAction != nil {
			if err := gb.setApproverDecision(ctx,
				id,
				AutoApprover,
				approverUpdateParams{
					Decision: decisionFromAutoAction(*gb.State.AutoAction),
					Comment:  AutoActionComment,
				}); err != nil {
				l.WithError(err).Error("couldn't set auto decision")
				return false, err
			}
		} else {
			if err := gb.dumpCurrState(ctx, id); err != nil {
				l.WithError(err).Error("couldn't dump state with id: " + id.String())
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

	gb.State = &state

	if state.Type == script.ApproverTypeFromSchema {
		// get approver from application body
		var allVariables map[string]interface{}
		allVariables, err = runCtx.GrabStorage()
		if err != nil {
			return errors.Wrap(err, "Unable to grab variables storage")
		}

		approvers := make(map[string]struct{})
		for approverVariableRef := range gb.State.Approvers {
			if len(strings.Split(approverVariableRef, dotSeparator)) == 1 {
				continue
			}
			approverVar := getVariable(allVariables, approverVariableRef)

			if approverVar == nil {
				return errors.Wrap(err, "Unable to find approver by variable reference")
			}

			if actualApproverUsername, castOK := approverVar.(string); castOK {
				approvers[actualApproverUsername] = gb.State.Approvers[approverVariableRef]
			}
		}

		if len(approvers) != 0 {
			gb.State.Approvers = approvers
			gb.State.LeftToNotify = approvers
		}
	}

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
	key := rejectedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ApproverDecisionApproved {
		key = approvedSocketID
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		key = editAppSocketID
	}

	if gb.State != nil && gb.State.Decision == nil && len(gb.State.AddInfo) != 0 {
		key = requestAddInfoSocketID
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoApproverBlock) Skipped(_ *store.VariableStore) []string {
	key := approvedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ApproverDecisionApproved {
		key = rejectedSocketID
	}
	var nexts, ok = script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil
	}

	return nexts
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
				ApproversGroupID:   "",
				ApproversGroupName: "",
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.ApprovedSocket,
			script.RejectedSocket,
			script.EditAppSocket,
		},
	}
}
