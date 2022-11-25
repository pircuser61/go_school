package pipeline

import (
	"context"
	"time"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoTestBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket

	RunContext *BlockRunContext
}

func (gb *GoTestBlock) Members() map[string]struct{} {
	return nil
}

func (gb *GoTestBlock) CheckSLA() (bool, bool, time.Time) {
	return false, false, time.Time{}
}

func (gb *GoTestBlock) UpdateManual() bool {
	return false
}

func (gb *GoTestBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoTestBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *GoTestBlock) GetType() string {
	return BlockGoTestID
}

func (gb *GoTestBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoTestBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoTestBlock) IsScenario() bool {
	return false
}

// nolint:dupl // not dupl?
func (gb *GoTestBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
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

	return nil
}

func (gb *GoTestBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoTestBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoTestBlock) GetState() interface{} {
	return nil
}

func (gb *GoTestBlock) Update(_ context.Context) (interface{}, error) {
	return nil, nil
}

func createGoTestBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) *GoTestBlock {
	b := &GoTestBlock{
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
	return b
}
