package pipeline

import (
	"context"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type SyncData struct {
	IncomingBlockIds []string `json:"incoming_block_ids"`
	done             bool
}

type GoWaitForAllInputsBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string

	State *SyncData

	Pipeline *ExecutablePipeline
}

func (gb *GoWaitForAllInputsBlock) GetStatus() Status {
	if gb.State.done {
		return StatusFinished
	}
	return StatusRunning
}

func (gb *GoWaitForAllInputsBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoWaitForAllInputsBlock) GetType() string {
	return BlockWaitForAllInputsId
}

func (gb *GoWaitForAllInputsBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoWaitForAllInputsBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoWaitForAllInputsBlock) IsScenario() bool {
	return false
}

func (gb *GoWaitForAllInputsBlock) DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_go_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	executed, err := gb.Pipeline.Storage.CheckTaskStepsExecuted(ctx, stepCtx.workNumber, gb.State.IncomingBlockIds)
	if err != nil {
		return err
	}
	gb.State.done = executed

	return nil
}

func (gb *GoWaitForAllInputsBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := gb.Nexts[DefaultSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoWaitForAllInputsBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoWaitForAllInputsBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoWaitForAllInputsBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoWaitForAllInputsBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockWaitForAllInputsId,
		BlockType: script.TypeGo,
		Title:     BlockGoWaitForAllInputsTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []string{DefaultSocket},
	}
}

func getInputBlocks(pipeline *ExecutablePipeline, name string) (entries []string) {
	var handleKey func(key string)
	handleKey = func(key string) {
		for socketName, bb := range pipeline.PipelineModel.Pipeline.Blocks[key].Next {
			if socketName == editAppSocket {
				continue
			}

			addKey := false
			for _, nextBlockName := range bb {
				if nextBlockName == name {
					addKey = true
					continue
				}
				handleKey(nextBlockName)
			}
			if addKey {
				entries = append(entries, key)
			}
		}
	}
	handleKey(pipeline.EntryPoint)

	entries = removeDuplicateStr(entries)

	return entries
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := make([]string, 0)
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func createGoWaitForAllInputsBlock(name string, ef *entity.EriusFunc, pipeline *ExecutablePipeline) *GoWaitForAllInputsBlock {
	b := &GoWaitForAllInputsBlock{
		Name:     name,
		Title:    ef.Title,
		Input:    map[string]string{},
		Output:   map[string]string{},
		Nexts:    ef.Next,
		State:    &SyncData{IncomingBlockIds: getInputBlocks(pipeline, name)},
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
