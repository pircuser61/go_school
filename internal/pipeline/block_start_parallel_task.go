package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type SyncDataStart struct {
	OutcomeBlockIds []string `json:"outcoming_block_ids"`
	done            bool
}

type GoStartParallelTaskBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string

	State *SyncDataStart

	Pipeline *ExecutablePipeline
}

func (gb *GoStartParallelTaskBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoStartParallelTaskBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoStartParallelTaskBlock) GetType() string {
	return BlockWaitForAllInputsId
}

func (gb *GoStartParallelTaskBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoStartParallelTaskBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoStartParallelTaskBlock) IsScenario() bool {
	return false
}

func (gb *GoStartParallelTaskBlock) DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_go_block")
	defer s.End()

	runCtx.AddStep(gb.Name)
	return nil
}

func (gb *GoStartParallelTaskBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := gb.Nexts[DefaultSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoStartParallelTaskBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoStartParallelTaskBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoStartParallelTaskBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoStartParallelTaskBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoStartParallelTaskId,
		BlockType: script.TypeGo,
		Title:     BlockGoStartParallelTaskTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []string{DefaultSocket},
	}
}
