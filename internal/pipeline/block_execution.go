package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputExecutionType     = "type"
	keyOutputExecutionLogin    = "login"
	keyOutputExecutionDecision = "decision"
	keyOutputExecutionComment  = "comment"

	ExecutionDecisionExecuted    ExecutionDecision = "executed"
	ExecutionDecisionNotExecuted ExecutionDecision = "not_executed"
)

type ExecutionUpdateParams struct {
	Decision ExecutionDecision `json:"decision"`
	Comment  string            `json:"comment"`
}

type ExecutionDecision string

func (a ExecutionDecision) String() string {
	return string(a)
}

type ExecutionData struct {
	ExecutionType  script.ExecutionType `json:"execution_type"`
	Executors      map[string]struct{}  `json:"executors"`
	Decision       *ExecutionDecision   `json:"decision,omitempty"`
	Comment        *string              `json:"comment,omitempty"`
	ActualExecutor *string              `json:"actual_executor,omitempty"`
	SLA            int                  `json:"sla"`
}

func (a *ExecutionData) GetDecision() *ExecutionDecision {
	return a.Decision
}

func (a *ExecutionData) SetDecision(login string, decision ExecutionDecision, comment string) error {
	_, ok := a.Executors[login]
	if !ok {
		return fmt.Errorf("%s not found in executors", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if decision != ExecutionDecisionExecuted && decision != ExecutionDecisionNotExecuted {
		return fmt.Errorf("unknown decision %s", decision.String())
	}

	a.Decision = &decision
	a.Comment = &comment
	a.ActualExecutor = &login

	return nil
}

type GoExecutionBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string
	State  *ExecutionData

	Pipeline *ExecutablePipeline
}

func (gb *GoExecutionBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusExecuted
		}
		return StatusExecutionRejected
	}

	return StatusExecution
}

func (gb *GoExecutionBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusFinished
		}
		return StatusNoSuccess
	}
	return StatusRunning
}

func (gb *GoExecutionBlock) GetTaskStatus() TaskHumanStatus {
	return StatusNew
}

func (gb *GoExecutionBlock) GetType() string {
	return BlockGoExecutionID
}

func (gb *GoExecutionBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoExecutionBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoExecutionBlock) IsScenario() bool {
	return false
}

func (gb *GoExecutionBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_execution_block")
	defer s.End()

	// TODO: fix
	// runCtx.AddStep(gb.Name)

	// TODO: handle SLA

	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	var step *entity.Step
	step, err = gb.Pipeline.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	} else if step == nil {
		// still waiting
		return nil
	}

	data, ok := step.State[gb.Name]
	if !ok {
		return nil
	}

	var state ExecutionData
	if err = json.Unmarshal(data, &state); err != nil {
		return errors.Wrap(err, "invalid format of go-execution-block state")
	}

	gb.State = &state

	decision := gb.State.GetDecision()

	// nolint:dupl // not dupl?
	if decision != nil {
		var executor, comment string

		if state.ActualExecutor != nil {
			executor = *state.ActualExecutor
		}

		if state.Comment != nil {
			comment = *state.Comment
		}

		runCtx.SetValue(gb.Output[keyOutputExecutionLogin], executor)
		runCtx.SetValue(gb.Output[keyOutputExecutionDecision], decision.String())
		runCtx.SetValue(gb.Output[keyOutputExecutionComment], comment)

		var stateBytes []byte
		stateBytes, err = json.Marshal(gb.State)
		if err != nil {
			return err
		}

		runCtx.ReplaceState(gb.Name, stateBytes)
	}

	return err
}

func (gb *GoExecutionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := notExecutedSocket
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = executedSocket
	}
	nexts, ok := gb.Nexts[key]
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoExecutionBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoExecutionBlock) Update(ctx context.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("update data is empty")
	}

	var updateParams ExecutionUpdateParams
	err := json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return nil, errors.New("can't assert provided update data")
	}

	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, data.Id)
	if err != nil {
		return nil, err
	} else if step == nil {
		return nil, errors.New("can't get step from database")
	}

	stepData, ok := step.State[gb.Name]
	if !ok {
		return nil, errors.New("can't get step state")
	}

	var state ExecutionData
	if err = json.Unmarshal(stepData, &state); err != nil {
		return nil, errors.Wrap(err, "invalid format of go-execution-block state")
	}

	gb.State = &state

	if errSet := gb.State.SetDecision(
		data.ByLogin,
		updateParams.Decision,
		updateParams.Comment,
	); errSet != nil {
		return nil, errSet
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	content, err := json.Marshal(step)
	if err != nil {
		return nil, err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          data.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusFinished),
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (gb *GoExecutionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoExecutionID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputExecutionType,
				Type:    "string",
				Comment: "execution type (user, group)",
			},
			{
				Name:    keyOutputExecutionLogin,
				Type:    "string",
				Comment: "executor login",
			},
			{
				Name:    keyOutputExecutionDecision,
				Type:    "string",
				Comment: "execution status",
			},
			{
				Name:    keyOutputExecutionComment,
				Type:    "string",
				Comment: "execution status comment",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoExecutionID,
			Params: &script.ExecutionParams{
				Executors: "",
				Type:      "",
				SLA:       0,
			},
		},
		Sockets: []string{executedSocket, notExecutedSocket},
	}
}

// nolint:dupl // another block
func createGoExecutionBlock(name string, ef *entity.EriusFunc, pipeline *ExecutablePipeline) (*GoExecutionBlock, error) {
	b := &GoExecutionBlock{
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

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.ExecutionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get execution parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid execution parameters")
	}

	b.State = &ExecutionData{
		ExecutionType: params.Type,
		Executors:     map[string]struct{}{params.Executors: {}},
		SLA:           params.SLA,
	}

	return b, nil
}
