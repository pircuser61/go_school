package pipeline

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"go.opencensus.io/trace"
)

type Pipeline struct {
	ID       uuid.UUID           `json:"id,omitempty"`
	Name     string              `json:"name"`
	Input    []map[string]string `json:"input"`
	Output   []map[string]string `json:"output"`
	Pipeline *ExecutablePipeline `json:"pipeline"`
}

func NewPipeline(model db.PipelineStorageModelDepricated, connection *dbconn.PGConnection) (*Pipeline, error) {
	p := Pipeline{}
	b := []byte(model.Pipeline)
	if len(b) == 0 {
		return nil, errors.New("unknown pipeline")
	}
	err := json.Unmarshal(b, &p)
	if err != nil {
		return nil, errors.Errorf("can't unmarshal pipeline: %s", err.Error())
	}
	p.ID, p.Pipeline.PipelineID = model.ID, model.ID
	p.Pipeline.Storage = connection
	return &p, nil
}

func (p *Pipeline) Run(ctx context.Context, runCtx *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_pipeline")
	defer s.End()
	startContext := NewStore()
	for _, inputValue := range p.Input {
		glob, ok := inputValue["global"]
		if !ok {
			return errors.New("can't find global variable name")
		}
		loc, ok := inputValue["name"]
		if !ok {
			return errors.New("can't find variable name")
		}
		inputVal, err := runCtx.GetString(loc)
		if err != nil {
			return err
		}
		startContext.SetValue(glob, inputVal)
	}
	return p.Pipeline.Run(ctx, &startContext)
}

func (p *Pipeline) ReturnOutput() (map[string]interface{}, error) {
	out := make(map[string]interface{})
	for _, v := range p.Output {
		globalKey, ok := v["global"]
		if !ok {
			return nil, errors.New("can't find global variable name")
		}

		val, err := p.Pipeline.VarStore.GetValue(globalKey)
		if err != nil {
			return nil, errors.Errorf("can't find returning variable value: %s", err.Error())
		}

		name, ok := v["name"]
		if !ok {
			return nil, errors.New("can't find global variable name")
		}

		out[name] = val
	}
	return out, nil
}
