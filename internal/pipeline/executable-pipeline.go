package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
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
	fmt.Println(ep.Blocks)
	for ep.NowOnPoint != "" {
		fmt.Println("executing", ep.NowOnPoint)
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
	ep.Blocks = make(map[string]Runner)
	for k, block := range source {
		fmt.Println(block.BlockType)
		switch block.BlockType {
		case "internal", "term":
			ep.Blocks[k] = CreateInternal(block, k)
		case "python3":
			fb := FunctionBlock{
				BlockName:      k,
				FunctionName:   block.Title,
				FunctionInput:  make(map[string]string),
				FunctionOutput: make(map[string]string),
				NextStep:       block.Next,
				runURL: "manager",
			}
			for _, v := range block.Input {
				fb.FunctionInput[v.Name] = v.Global
			}
			for _, v := range block.Output {
				fb.FunctionOutput[v.Name] = v.Global
			}
			ep.Blocks[k] = &fb
		}
	}
	return nil
}

func CreateInternal(ef entity.EriusFunc, name string) Runner {
	switch ef.Title {
	case "input":
		i :=  InputBlock{
			BlockName:     name,
			FunctionName:  ef.Title,
			NextStep:     ef.Next,
			FunctionInput: make(map[string]string),
		}
		for _, v := range ef.Output {
			i.FunctionInput[v.Name] = v.Global
		}
		return &i
	case "if":
		i := IF{
			BlockName: name,
			FunctionName:  ef.Title,
			OnTrue:        ef.OnTrue,
			OnFalse:       ef.OnFalse,
			FunctionInput: make(map[string]string),
		}
		for _, v := range ef.Input {
			i.FunctionInput[v.Name] = v.Global
		}
		return &i
	case "strings_is_equal":
		sie := StringsEqual{
			BlockName: name,
			FunctionName:  ef.Title,
			OnTrue:        ef.OnTrue,
			OnFalse:       ef.OnFalse,
			FunctionInput: make(map[string]string),
		}
		for _, v := range ef.Input {
			sie.FunctionInput[v.Name] = v.Global
		}
		return &sie
	}
	return nil
}
