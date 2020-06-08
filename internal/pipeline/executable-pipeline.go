package pipeline

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"go.opencensus.io/trace"
)

const (
	kind = "kind"
	faas = "faas"
)

type ExecutablePipeline struct {
	WorkId     uuid.UUID
	PipelineID uuid.UUID
	VersionID uuid.UUID
	Storage    *dbconn.PGConnection
	Entrypoint string
	NowOnPoint string
	VarStore   *VariableStore
	Blocks     map[string]Runner
	NextStep   string
}

func (ep *ExecutablePipeline) CreateWork(ctx context.Context, author string) error {
	ep.WorkId = uuid.New()
	err := db.WriteTask(ctx, ep.Storage, ep.WorkId, ep.VersionID, author)
	if err != nil {
		return err
	}
	return nil
}

func (ep *ExecutablePipeline) Run(ctx context.Context, runCtx *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "pipeline_flow")
	defer s.End()
	ep.VarStore = runCtx
	if ep.NowOnPoint == "" {
		ep.NowOnPoint = ep.Entrypoint
	}
	for ep.NowOnPoint != "" {
		err := ep.Blocks[ep.NowOnPoint].Run(ctx, ep.VarStore)
		if err != nil {
			errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkId, db.RunStatusError)
			if errChange != nil {
				return errChange
			}
			return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
		}
		storageData, err := json.Marshal(ep.VarStore)
		if err != nil {
			errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkId, db.RunStatusError)
			if errChange != nil {
				return errChange
			}
			return err
		}
		err = db.WriteContext(ctx, ep.Storage, ep.WorkId,
			ep.PipelineID, ep.NowOnPoint, storageData)
		ep.NowOnPoint = ep.Blocks[ep.NowOnPoint].Next()
		if err != nil {
			errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkId, db.RunStatusError)
			if errChange != nil {
				return errChange
			}
			return err
		}
	}
	err := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkId, db.RunStatusFinished)
	if err != nil {
		return err
	}
	return nil
}

func (ep *ExecutablePipeline) Next() string {
	return ep.NextStep
}

func (ep *ExecutablePipeline) CreateBlocks(source map[string]entity.EriusFunc) error {
	for k, v := range source {
		ep.Blocks = make(map[string]Runner)
		switch v.BlockType {
		case "internal":
			ep.Blocks[k] = CreateInternal(v)
		}
	}
	return nil
}

func CreateInternal(ef entity.EriusFunc) Runner {
	switch ef.Title {
	case "input":
		i :=  InputBlock{
			BlockName:     ef.Title,
			FunctionName:  ef.Title,
			NextStep:     ef.Next,
			FunctionInput: make(map[string]string),
		}
		for _, v := range ef.Output {
			i.FunctionInput[v.Name] = v.Global
		}
		return &i
	}
	return nil
}
