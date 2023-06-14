package pipeline

import (
	"context"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoStartBlock struct {
	Name       string
	Title      string
	Input      map[string]string
	Output     map[string]string
	Sockets    []script.Socket
	RunContext *BlockRunContext
}

func (gb *GoStartBlock) Members() []Member {
	return nil
}

func (gb *GoStartBlock) Deadlines() []Deadline {
	return []Deadline{}
}

func (gb *GoStartBlock) UpdateManual() bool {
	return false
}

func (gb *GoStartBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoStartBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoStartBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoStartBlock) GetState() interface{} {
	return nil
}

func (gb *GoStartBlock) Update(ctx context.Context) (interface{}, error) {
	data, err := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if err != nil {
		return nil, errors.Wrap(err, "can't get task run context")
	}

	personData, err := gb.RunContext.ServiceDesc.GetSsoPerson(ctx, gb.RunContext.Initiator)
	if err != nil {
		return nil, err
	}

	for k := range gb.Output {
		val, ok := data.InitialApplication.ApplicationBody.Get(k)
		if !ok {
			continue
		}
		gb.RunContext.VarStore.SetValue(gb.Output[k], val)
	}

	gb.RunContext.VarStore.SetValue(gb.Output[entity.KeyOutputWorkNumber], gb.RunContext.WorkNumber)
	gb.RunContext.VarStore.SetValue(gb.Output[entity.KeyOutputApplicationInitiator], personData)

	return nil, nil
}

func (gb *GoStartBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoStartId,
		BlockType: script.TypeGo,
		Title:     BlockGoStartTitle,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    entity.KeyOutputWorkNumber,
				Type:    "string",
				Comment: "work number",
			},
			{
				Name:    entity.KeyOutputApplicationInitiator,
				Type:    "SsoPerson",
				Comment: "task initiator",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

//nolint:dupl //its not duplicate
func createGoStartBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoStartBlock, bool, error) {
	b := &GoStartBlock{
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

	return b, false, nil
}
