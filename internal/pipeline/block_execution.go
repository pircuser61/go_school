package pipeline

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputExecutionType    = "type"
	keyOutputExecutionLogin   = "login"
	keyOutputExecutionStatus  = "status"
	keyOutputExecutionComment = "comment"
)

type ExecutionData struct {
	ExecutionType script.ExecutionType `json:"execution_type"`
	Executors     map[string]struct{}  `json:"executors"`
}

type GoExecutionBlock struct {
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep []string
	State    *ExecutionData

	Storage db.Database
}

func (gb *GoExecutionBlock) GetTaskHumanStatus() TaskHumanStatus {
	return StatusExecution
}

func (gb *GoExecutionBlock) GetStatus() Status {
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

func (gb *GoExecutionBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

func (gb *GoExecutionBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_execution_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	_, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	// TODO add executors to application(s) here

	return err
}

func (gb *GoExecutionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return gb.NextStep, true
}

func (gb *GoExecutionBlock) NextSteps() []string {
	return gb.NextStep
}

func (gb *GoExecutionBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoExecutionBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoExecutionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoExecutionID,
		BlockType: script.TypeGo,
		Title:     BlockGoExecutionTitle,
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
				Name:    keyOutputExecutionStatus,
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
				Executors: []string{},
				Type:      "",
			},
		},
		NextFuncs: []string{script.Next},
	}
}

func createGoExecutionBlock(name string, ef *entity.EriusFunc, storage db.Database) (*GoExecutionBlock, error) {
	b := &GoExecutionBlock{
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

	executors := map[string]struct{}{}
	for i := range params.Executors {
		executors[params.Executors[i]] = struct{}{}
	}

	b.State = &ExecutionData{
		ExecutionType: params.Type,
		Executors:     executors,
	}

	return b, nil
}
