package pipeline

import (
	"context"
	"fmt"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoEndBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string

	Pipeline *ExecutablePipeline
}

func (gb *GoEndBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoEndBlock) GetTaskHumanStatus() TaskHumanStatus {
	return "" // should not change status returned by worker nodes like approvement, execution, etc.
}

func (gb *GoEndBlock) GetType() string {
	return BlockGoEndId
}

func (gb *GoEndBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoEndBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoEndBlock) IsScenario() bool {
	return false
}

// nolint:dupl // not dupl?
func (gb *GoEndBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_go_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	values := make(map[string]interface{})

	for ikey, gkey := range gb.Input {
		val, ok := runCtx.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	for ikey, gkey := range gb.Output {
		val, ok := values[ikey]
		if ok {
			runCtx.SetValue(gkey, val)
		}
	}

	gb.updateTaskStatus(ctx, gb.Pipeline)

	return nil
}

func (gb *GoEndBlock) updateTaskStatus(ctx context.Context, pipeline *ExecutablePipeline) {
	entries := getInputBlocks(pipeline, gb.Name)
	if len(entries) == 0 {
		fmt.Println("end len(entries) == 0 updateTaskStatus")
		return
	}

	step, err := pipeline.Storage.GetTaskStepByName(ctx, gb.Pipeline.TaskID, entries[0])
	if err != nil {
		fmt.Println(err, "end updateTaskStatus")
		return
	}

	if step != nil && step.Status == string(StatusNoSuccess) && step.Type == BlockGoApproverID {
		err = gb.Pipeline.updateStatusByStep(ctx, StatusApprovementRejected)
		if err != nil {
			fmt.Println(err, "end updateTaskStatus")
			return
		}
	}

	if step != nil && step.Status == string(StatusNoSuccess) && step.Type == BlockGoExecutionID {
		err = gb.Pipeline.updateStatusByStep(ctx, StatusExecutionRejected)
		if err != nil {
			fmt.Println(err, "end updateTaskStatus")
			return
		}
	}
}

func (gb *GoEndBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return nil, true
}

func (gb *GoEndBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoEndBlock) GetState() interface{} {
	return nil
}

func (gb *GoEndBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoEndBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoEndId,
		BlockType: script.TypeGo,
		Title:     BlockGoEndTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []string{}, // TODO: по идее, тут нет никаких некстов, возможно, в будущем они понадобятся
	}
}

func createGoEndBlock(name string, ef *entity.EriusFunc, pipeline *ExecutablePipeline) *GoEndBlock {
	b := &GoEndBlock{
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
	return b
}