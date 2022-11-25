package pipeline

import (
	"context"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputWorkNumber = "workNumber"
)

type GoStartBlock struct {
	Name       string
	Title      string
	Input      map[string]string
	Output     map[string]string
	Sockets    []script.Socket
	RunContext *BlockRunContext
}

func (gb *GoStartBlock) Members() map[string]struct{} {
	return nil
}

func (gb *GoStartBlock) CheckSLA() (bool, bool, time.Time) {
	return false, false, time.Time{}
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

func (gb *GoStartBlock) Update(_ context.Context) (interface{}, error) {
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputWorkNumber], gb.RunContext.WorkNumber)
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
				Name:    keyOutputWorkNumber,
				Type:    "string",
				Comment: "work number",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func createGoStartBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) *GoStartBlock {
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

	return b
}
