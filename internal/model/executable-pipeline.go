package model

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"go.opencensus.io/trace"
)

const (
	kind         = "kind"
	faas         = "faas"
	stringsEqual = "strings_equal"
)

type ExecutablePipeline struct {
	WorkId     uuid.UUID
	PipelineID uuid.UUID
	Storage    *dbconn.PGConnection
	Entrypoint string
	NowOnPoint string
	VarStore   *VariableStore
	Blocks     map[string]Runner
	NextStep   string
}

func (ep *ExecutablePipeline) Run(ctx context.Context, runCtx *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "pipeline_flow")
	defer s.End()
	ep.WorkId = uuid.New()
	ep.VarStore = runCtx
	if ep.NowOnPoint == "" {
		ep.NowOnPoint = ep.Entrypoint
	}
	for ep.NowOnPoint != "" {
		err := ep.Blocks[ep.NowOnPoint].Run(ctx, ep.VarStore)
		if err != nil {
			return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
		}
		storageData, err := json.Marshal(ep.VarStore)
		if err != nil {
			return err
		}
		err = db.WriteContext(ctx, ep.Storage, ep.WorkId,
			ep.PipelineID, ep.NowOnPoint, storageData)
		ep.NowOnPoint = ep.Blocks[ep.NowOnPoint].Next()
	}

	return nil
}

func (ep *ExecutablePipeline) Next() string {
	return ep.NextStep
}

func (ep *ExecutablePipeline) UnmarshalJSON(b []byte) error {
	p := make(map[string]interface{})
	err := json.Unmarshal(b, &p)
	if err != nil {
		return err
	}
	for k, v := range p {
		switch v.(type) {
		case string:
			switch k {
			case "entrypoint":
				pEntry, ok := v.(string)
				if !ok {
					return errors.New("can't parse entrypoint")
				}
				ep.Entrypoint = pEntry
			}
		case map[string]interface{}:
			switch k {
			case "blocks":
				blocksMap, ok := v.(map[string]interface{})
				if !ok {
					return errors.New("can't parse blocks")
				}
				blocks, err := UnmarshalBlocks(blocksMap)
				if err != nil {
					return errors.Errorf("can't unmarshal function blocks: %s", err.Error())
				}
				ep.Blocks = blocks
			}
		}
	}
	return nil
}

func UnmarshalBlocks(m map[string]interface{}) (map[string]Runner, error) {
	blocks := make(map[string]Runner)
	for k, v := range m {
		b, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("can't parse block %s", k)
		}
		switch b[kind] {
		case faas:
			block, err := NewFunction(k, b)
			if err != nil {
				return nil, errors.Errorf("can't create function: %s", err.Error())
			}
			blocks[k] = block
		case "if":
			block, err := NewIF(b)
			if err != nil {
				return nil, errors.Errorf("can't create function: %s", err.Error())
			}
			blocks[k] = block
		}
	}

	return blocks, nil
}
