package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoPlaceholderBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket

	RunContext *BlockRunContext
}

func (gb *GoPlaceholderBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockPlaceholderID,
		BlockType: script.TypeGo,
		Title:     BlockPlaceholderTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []script.Socket{script.DefaultSocket},
		Params: &script.FunctionParams{
			Type: BlockPlaceholderID,
			Params: &script.PlaceholderParams{
				Name:        "",
				Description: "",
			},
		},
	}
}

func (gb *GoPlaceholderBlock) Members() []Member {
	return nil
}

func (gb *GoPlaceholderBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoPlaceholderBlock) UpdateManual() bool {
	return false
}

func (gb *GoPlaceholderBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoPlaceholderBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoPlaceholderBlock) GetType() string {
	return BlockPlaceholderID
}

func (gb *GoPlaceholderBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoPlaceholderBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoPlaceholderBlock) IsScenario() bool {
	return false
}

func (gb *GoPlaceholderBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoPlaceholderBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoPlaceholderBlock) GetState() interface{} {
	return nil
}

func (gb *GoPlaceholderBlock) Update(_ context.Context) (interface{}, error) {
	return nil, nil
}

//nolint:unparam // its ok
func createGoPlaceholderBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoPlaceholderBlock, bool, error) {
	const reEntry = false

	b := &GoPlaceholderBlock{
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

	b.RunContext.VarStore.AddStep(b.Name)

	return b, reEntry, nil
}
