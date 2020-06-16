package pipeline

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

type BlockName string

type Pipeline struct {
	ID       uuid.UUID           `json:"id,omitempty"`
	Name     string              `json:"name"`
	Input    []map[string]string `json:"input"`
	Output   []map[string]string `json:"output"`
	Pipeline *ExecutablePipeline `json:"pipeline"`
}

var (
	errCantFindGlobalVarName = errors.New("can't find global variable name")
	errCantFindVarName       = errors.New("can't find variable name")
)

func (p *Pipeline) Run(ctx context.Context, runCtx *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_pipeline")
	defer s.End()

	startContext := NewStore()

	for _, inputValue := range p.Input {
		glob, ok := inputValue["global"]
		if !ok {
			return errCantFindGlobalVarName
		}

		loc, ok := inputValue["name"]
		if !ok {
			return errCantFindVarName
		}

		inputVal, err := runCtx.GetString(loc)
		if err != nil {
			return err
		}

		startContext.SetValue(glob, inputVal)
	}

	return p.Pipeline.Run(ctx, &startContext)
}
