package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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

type Approver struct {
	Decision *ApproverDecision `json:"decision,omitempty"`
	Comment  *string           `json:"comment,omitempty"`
}

type ApproverData struct {
	Type           script.ApproverType `json:"type"`
	Approvers      map[string]struct{} `json:"approvers"`
	Decision       *ApproverDecision   `json:"decision,omitempty"`
	Comment        *string             `json:"comment,omitempty"`
	ActualApprover *string             `json:"actual_approver,omitempty"`
}

func (a *ApproverData) GetDecision() *ApproverDecision {
	return a.Decision
}

func (a *ApproverData) SetDecision(login string, decision ApproverDecision, comment string) error {
	_, ok := a.Approvers[login]
	if !ok {
		return fmt.Errorf("%s not found in approvers", login)
	}

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
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep []string
	State    *ApproverData

	Storage db.Database
}

func (gb *GoApproverBlock) GetTaskStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.Decision != nil {
		return StatusApproved
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

func (gb *GoApproverBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

func (gb *GoApproverBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_approver_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	var waitTime time.Duration
	var decision *ApproverDecision

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(waitTime):
			// update waiting time
			waitTime = time.Second * 10

			// check state from database
			var step *entity.Step
			step, err = gb.Storage.GetTaskStepById(ctx, id)
			if err != nil {
				return err
			} else if step == nil {
				// still waiting
				continue
			}

			// get state from step.State
			data, ok := step.State[gb.Name]
			if !ok {
				continue
			}

			var state ApproverData
			err = json.Unmarshal(data, &state)
			if err != nil {
				return errors.Wrap(err, "invalid format of go-approver-block state")
			}

			gb.State = &state

			// check decision
			decision = gb.State.GetDecision()
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

				return nil
			}
		}
	}
}

func (gb *GoApproverBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return gb.NextStep, true
}

func (gb *GoApproverBlock) NextSteps() []string {
	return gb.NextStep
}

func (gb *GoApproverBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoApproverBlock) Update(ctx context.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}

	var updateParams ApproverUpdateParams
	err := json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return nil, errors.New("can't assert provided data")
	}

	step, err := gb.Storage.GetTaskStepById(ctx, data.Id)
	if err != nil {
		return nil, err
	} else if step == nil {
		return nil, errors.New("can't get step from database")
	}

	// get state from step.State
	stepData, ok := step.State[gb.Name]
	if !ok {
		return nil, errors.New("can't get step state")
	}

	var state ApproverData
	err = json.Unmarshal(stepData, &state)
	if err != nil {
		return nil, errors.Wrap(err, "invalid format of go-approver-block state")
	}

	gb.State = &state

	err = gb.State.SetDecision(
		data.ByLogin,
		updateParams.Decision,
		updateParams.Comment,
	)
	if err != nil {
		return nil, err
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	content, err := json.Marshal(step)
	if err != nil {
		return nil, err
	}

	err = gb.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          data.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		IsFinished:  false,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (gb *GoApproverBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoApproverID,
		BlockType: script.TypeGo,
		Title:     BlockGoApproverTitle,
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
				Approver: "",
				Type:     "",
			},
		},
		NextFuncs: []string{script.Next},
	}
}

func createGoApproverBlock(name string, ef *entity.EriusFunc, storage db.Database) (*GoApproverBlock, error) {
	b := &GoApproverBlock{
		Storage: storage,

		Name:     name,
		Title:    ef.Title,
		Input:    map[string]string{},
		Output:   map[string]string{},
		NextStep: ef.Next,
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
	}

	return b, nil
}
