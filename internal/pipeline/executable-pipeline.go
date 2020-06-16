package pipeline

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/libs/logger"
	"go.opencensus.io/trace"
)

type ExecutablePipeline struct {
	WorkID     uuid.UUID
	PipelineID uuid.UUID
	VersionID  uuid.UUID
	Storage    *dbconn.PGConnection
	Entrypoint BlockName
	NowOnPoint BlockName
	VarStore   *VariableStore
	Blocks     map[BlockName]Runner
	NextStep   BlockName

	Logger logger.Logger
}

func (ep *ExecutablePipeline) CreateWork(ctx context.Context, author string) error {
	ep.WorkID = uuid.New()

	err := db.WriteTask(ctx, ep.Storage, ep.WorkID, ep.VersionID, author)
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
		ep.Logger.Println("executing", ep.NowOnPoint)

		err := ep.Blocks[ep.NowOnPoint].Run(ctx, ep.VarStore)
		if err != nil {
			errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkID, db.RunStatusError)
			if errChange != nil {
				return errChange
			}

			return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
		}

		storageData, err := json.Marshal(ep.VarStore)
		if err != nil {
			errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkID, db.RunStatusError)
			if errChange != nil {
				return errChange
			}

			return err
		}

		err = db.WriteContext(ctx, ep.Storage, ep.WorkID, string(ep.NowOnPoint), storageData)
		ep.NowOnPoint = ep.Blocks[ep.NowOnPoint].Next()

		if err != nil {
			errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkID, db.RunStatusError)
			if errChange != nil {
				return errChange
			}

			return err
		}
	}

	err := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkID, db.RunStatusFinished)
	if err != nil {
		return err
	}

	return nil
}

func (ep *ExecutablePipeline) Next() BlockName {
	return ep.NextStep
}

func (ep *ExecutablePipeline) CreateBlocks(source map[string]entity.EriusFunc) error {
	ep.Blocks = make(map[BlockName]Runner)

	for k := range source {
		bn := BlockName(k)

		block := source[k]
		switch block.BlockType {
		case "internal", "term":
			ep.Blocks[bn] = CreateInternal(&block, bn)
		case "python3":
			fb := FunctionBlock{
				Name:           bn,
				FunctionName:   block.Title,
				FunctionInput:  make(map[string]string),
				FunctionOutput: make(map[string]string),
				NextStep:       BlockName(block.Next),
				runURL:         "https://openfaas-staging.dev.autobp.mts.ru/function/%s.openfaas-fn",
			}

			for _, v := range block.Input {
				fb.FunctionInput[v.Name] = v.Global
			}

			for _, v := range block.Output {
				fb.FunctionOutput[v.Name] = v.Global
			}

			ep.Blocks[bn] = &fb
		}
	}

	return nil
}

func createInputBlock(title string, name, next BlockName) *InputBlock {
	return &InputBlock{
		BlockName:     name,
		FunctionName:  title,
		NextStep:      next,
		FunctionInput: make(map[string]string),
	}
}

func createOutputBlock(title string, name, next BlockName) *OutputBlock {
	return &OutputBlock{
		BlockName:      name,
		FunctionName:   title,
		NextStep:       next,
		FunctionOutput: make(map[string]string),
	}
}

func createIF(title string, name, onTrue, onFalse BlockName) *IF {
	return &IF{
		Name:          name,
		FunctionName:  title,
		OnTrue:        onTrue,
		OnFalse:       onFalse,
		FunctionInput: make(map[string]string),
	}
}

func createStringsEqual(title string, name, onTrue, onFalse BlockName) *StringsEqual {
	return &StringsEqual{
		Name:          name,
		FunctionName:  title,
		OnTrue:        onTrue,
		OnFalse:       onFalse,
		FunctionInput: make(map[string]string),
	}
}

func createConnectorBlock(title string, name, next BlockName) *ConnectorBlock {
	return &ConnectorBlock{
		Name:           name,
		FunctionName:   title,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
		NextStep:       next,
	}
}

func CreateInternal(ef *entity.EriusFunc, name BlockName) Runner {
	switch ef.Title {
	case "input":
		i := createInputBlock(ef.Title, name, BlockName(ef.Next))

		for _, v := range ef.Output {
			i.FunctionInput[v.Name] = v.Global
		}

		return i
	case "output":
		i := createOutputBlock(ef.Title, name, BlockName(ef.Next))
		for _, v := range ef.Output {
			i.FunctionOutput[v.Name] = v.Global
		}

		return i
	case "if":
		i := createIF(ef.Title, name, BlockName(ef.OnTrue), BlockName(ef.OnFalse))

		for _, v := range ef.Input {
			i.FunctionInput[v.Name] = v.Global
		}

		return i
	case "strings_is_equal":
		sie := createStringsEqual(ef.Title, name, BlockName(ef.OnTrue), BlockName(ef.OnFalse))

		for _, v := range ef.Input {
			sie.FunctionInput[v.Name] = v.Global
		}

		return sie
	case "connector":
		con := createConnectorBlock(ef.Title, name, BlockName(ef.Next))

		for _, v := range ef.Input {
			con.FunctionInput[v.Name] = v.Global
		}

		for _, v := range ef.Output {
			con.FunctionOutput[v.Name] = v.Global
		}

		return con
	}

	return nil
}
