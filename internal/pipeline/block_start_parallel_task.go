package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type SyncDataStart struct {
	OutcomeBlockIds []string `json:"outcoming_block_ids"`
	done            bool
}

type GoBeginParallelTaskBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string

	State *SyncDataStart

	Pipeline *ExecutablePipeline
}

func (gb *GoBeginParallelTaskBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoBeginParallelTaskBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoBeginParallelTaskBlock) GetType() string {
	return BlockWaitForAllInputsId
}

func (gb *GoBeginParallelTaskBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoBeginParallelTaskBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoBeginParallelTaskBlock) IsScenario() bool {
	return false
}

func (gb *GoBeginParallelTaskBlock) DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) error {
	runCtx.AddStep(gb.Name)
	return nil
}

func (gb *GoBeginParallelTaskBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := gb.Nexts[DefaultSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoBeginParallelTaskBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoBeginParallelTaskBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoBeginParallelTaskBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoBeginParallelTaskBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoBeginParallelTaskId,
		BlockType: script.TypeGo,
		Title:     BlockGoBeginParallelTaskTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []string{DefaultSocket},
	}
}
