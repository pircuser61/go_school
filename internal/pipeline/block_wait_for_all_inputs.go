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

type SyncData struct {
	IncomingBlockIds []string `json:"incoming_block_ids"`
	done             bool
	IsRevoked        bool `json:"is_revoked"`
}

type GoWaitForAllInputsBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket

	State *SyncData

	Pipeline *ExecutablePipeline
}

func (gb *GoWaitForAllInputsBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusCancel
	}
	if gb.State.done {
		return StatusFinished
	}
	return StatusRunning
}

func (gb *GoWaitForAllInputsBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusRevoke
	}
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
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
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

func (gb *GoWaitForAllInputsBlock) Update(ctx context.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}
	if data.Action == string(entity.TaskUpdateActionCancelApp) {
		step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, data.Id)
		if err != nil {
			return nil, err
		}

		if step == nil {
			return nil, errors.New("can't get step from database")
		}
		if errUpdate := gb.formCancelPipeline(ctx, data, step); errUpdate != nil {
			return nil, errUpdate
		}
		return nil, nil
	}
	return nil, nil
}

func (gb *GoWaitForAllInputsBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockWaitForAllInputsId,
		BlockType: script.TypeGo,
		Title:     BlockGoWaitForAllInputsTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []script.Socket{script.DefaultSocket},
	}
}

func getInputBlocks(pipeline *ExecutablePipeline, name string) (entries []string) {
	var handleKey func(key string)
	handleKey = func(key string) {
		for _, bb := range pipeline.PipelineModel.Pipeline.Blocks[key].Sockets {
			if bb.Id == editAppSocketID || bb.Id == requestAddInfoSocketID {
				continue
			}

			addKey := false
			for _, nextBlockName := range bb.NextBlockIds {
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
		Sockets:  entity.ConvertSocket(ef.Sockets),
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

func (gb *GoWaitForAllInputsBlock) formCancelPipeline(ctx context.Context, in *script.BlockUpdateData, step *entity.Step) (err error) {
	gb.State.IsRevoked = true

	if step.State[gb.Name], err = json.Marshal(gb.State); err != nil {
		return err
	}
	var content []byte
	if content, err = json.Marshal(store.NewFromStep(step)); err != nil {
		return err
	}
	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          in.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusCancel),
	})
	return err
}
