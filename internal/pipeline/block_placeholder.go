package pipeline

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

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

	PlaceholderName        string `json:"name"`
	PlaceholderDescription string `json:"description"`

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

func (gb *GoPlaceholderBlock) Deadlines() []Deadline {
	return []Deadline{}
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

func createGoPlaceholderBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoPlaceholderBlock, error) {
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

	err := json.Unmarshal(ef.Params, &b)
	if err != nil {
		return nil, errors.Wrap(err, "can not get placeholder parameters")
	}

	b.RunContext.VarStore.AddStep(b.Name)

	return b, nil
}
