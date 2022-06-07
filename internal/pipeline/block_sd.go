package pipeline

import (
	"context"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type SdData struct {}

type SdResult struct {}

type GoSdBlock struct {
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep string
	State    *SdData

	Storage db.Database
}

func (gb *GoSdBlock) GetType() string {
	return BlockGoSdID
}

func (gb *GoSdBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoSdBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoSdBlock) IsScenario() bool {
	return false
}

func (gb *GoSdBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

func (gb *GoSdBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_sd_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	return nil
}

func (gb *GoSdBlock) Next(_ *store.VariableStore) (string, bool) {
	return gb.NextStep, true
}

func (gb *GoSdBlock) NextSteps() []string {
	nextSteps := []string{gb.NextStep}

	return nextSteps
}

func (gb *GoSdBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSdBlock) Update(_ context.Context, _ interface{}) (interface{}, error) {
	return nil, nil
}

func (gb *GoSdBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoSdID,
		BlockType: script.TypeGo,
		Title:     BlockGoSdTitle,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    "",
				Type:    "string",
				Comment: "result",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoSdID,
		},
		NextFuncs: []string{script.Next},
	}
}

func createGoSdBlock(name string, ef *entity.EriusFunc, storage db.Database) (*GoSdBlock, error) {
	b := &GoSdBlock{
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

	return b, nil
}
