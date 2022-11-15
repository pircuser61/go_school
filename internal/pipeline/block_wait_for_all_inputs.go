package pipeline

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

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

	RunContext *BlockRunContext
}

func (gb *GoWaitForAllInputsBlock) Members() map[string]struct{} {
	return nil
}

func (gb *GoWaitForAllInputsBlock) CheckSLA() bool {
	return false
}

func (gb *GoWaitForAllInputsBlock) UpdateManual() bool {
	return false
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

func (gb *GoWaitForAllInputsBlock) DebugRun(_ context.Context, _ *stepCtx, _ *store.VariableStore) error {
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

func (gb *GoWaitForAllInputsBlock) Update(ctx context.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}
	if data.Action == string(entity.TaskUpdateActionCancelApp) {
		return nil, gb.formCancelPipeline(ctx)
	}
	executed, err := gb.RunContext.Storage.CheckTaskStepsExecuted(ctx, gb.RunContext.WorkNumber, gb.State.IncomingBlockIds)
	if err != nil {
		return nil, err
	}
	gb.State.done = executed

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

//TODO
func getInputBlocks(workNumber, name string) (entries []string) {
	return nil
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

func createGoWaitForAllInputsBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoWaitForAllInputsBlock, error) {
	b := &GoWaitForAllInputsBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		State:      &SyncData{IncomingBlockIds: getInputBlocks(runCtx.WorkNumber, name)},
		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	rawState, ok := runCtx.VarStore.State[name]
	if ok {
		if err := b.loadState(rawState); err != nil {
			return nil, err
		}
	} else {
		if err := b.createState(); err != nil {
			return nil, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, nil
}

func (gb *GoWaitForAllInputsBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

func (gb *GoWaitForAllInputsBlock) createState() error {
	gb.State = &SyncData{IncomingBlockIds: getInputBlocks(gb.RunContext.WorkNumber, gb.Name)}
	gb.RunContext.VarStore.AddStep(gb.Name)
	return nil
}

// nolint:dupl // another block
func (gb *GoWaitForAllInputsBlock) formCancelPipeline(ctx context.Context) (err error) {
	gb.State.IsRevoked = true
	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}
	if changeErr := gb.RunContext.changeTaskStatus(ctx, db.RunStatusFinished); changeErr != nil {
		return changeErr
	}

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
	return nil
}
