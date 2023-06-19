package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type SyncData struct {
	IncomingBlockIds []string `json:"incoming_block_ids"`
	Done             bool     `json:"done"`
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

func (gb *GoWaitForAllInputsBlock) Members() []Member {
	return nil
}

func (gb *GoWaitForAllInputsBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoWaitForAllInputsBlock) UpdateManual() bool {
	return false
}

func (gb *GoWaitForAllInputsBlock) GetStatus() Status {
	if gb.State.Done {
		return StatusFinished
	}
	return StatusRunning
}

func (gb *GoWaitForAllInputsBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoWaitForAllInputsBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoWaitForAllInputsBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoWaitForAllInputsBlock) Update(ctx context.Context) (interface{}, error) {
	// TODO ???
	executed, err := gb.RunContext.Storage.CheckTaskStepsExecuted(ctx, gb.RunContext.WorkNumber, gb.State.IncomingBlockIds)
	if err != nil {
		return nil, err
	}

	if !executed {
		return nil, nil
	}

	variableStorage, err := gb.RunContext.Storage.GetMergedVariableStorage(ctx, gb.RunContext.TaskID, gb.State.IncomingBlockIds)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore = variableStorage
	gb.State.Done = executed

	state, stateErr := json.Marshal(gb.GetState())
	if stateErr != nil {
		return nil, stateErr
	}
	gb.RunContext.VarStore.ReplaceState(gb.Name, state)

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

func createGoWaitForAllInputsBlock(ctx context.Context, name string, ef *entity.EriusFunc,
	runCtx *BlockRunContext) (*GoWaitForAllInputsBlock, bool, error) {

	const reEntry = false

	b := &GoWaitForAllInputsBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
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
			return nil, reEntry, err
		}
	} else {
		if err := b.createState(ctx); err != nil {
			return nil, reEntry, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, reEntry, nil
}

func (gb *GoWaitForAllInputsBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

func (gb *GoWaitForAllInputsBlock) createState(ctx context.Context) error {
	steps, err := gb.RunContext.Storage.GetTaskStepsToWait(ctx, gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return err
	}
	gb.State = &SyncData{IncomingBlockIds: steps}
	return nil
}
