package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/erius/pipeliner/internal/integration"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

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
	Entrypoint string
	NowOnPoint string
	VarStore   *store.VariableStore
	Blocks     map[string]Runner
	NextStep   string
	Input      map[string]string
	Output     map[string]string

	Logger logger.Logger
	FaaS   string
}

func(ep *ExecutablePipeline) Inputs() map[string]string {
	return ep.Input
}

func (ep *ExecutablePipeline)  Outputs() map[string]string {
	return ep.Output
}
func (ep *ExecutablePipeline)  IsScenario() bool {
	return true
}

func (ep *ExecutablePipeline) CreateWork(ctx context.Context, author string) error {
	ep.WorkID = uuid.New()

	err := db.WriteTask(ctx, ep.Storage, ep.WorkID, ep.VersionID, author)
	if err != nil {
		return err
	}

	return nil
}
func OutWithDeep(d int, data ...interface{})  {
	s := ""
	for i:=0; i<=d; i++ {
		s += "  "
	}
	fmt.Println(s, data)
}
func (ep *ExecutablePipeline) Run(ctx context.Context, runCtx *store.VariableStore, deep int) error {
	ctx, s := trace.StartSpan(ctx, "pipeline_flow")
	defer s.End()

	ep.VarStore = runCtx
	OutWithDeep(0, "varstore prev:   ", ep.VarStore)
	OutWithDeep(deep, deep, "pipeline:", ep.Blocks)
	if ep.NowOnPoint == "" {
		ep.NowOnPoint = ep.Entrypoint
	}
	for ep.NowOnPoint != "" {
		OutWithDeep(deep, ep.VarStore)
		OutWithDeep(deep, "now on", ep.NowOnPoint)
		ep.Logger.Println("executing", ep.NowOnPoint)
		if ep.Blocks[ep.NowOnPoint].IsScenario() {
			input := ep.Blocks[ep.NowOnPoint].Inputs()
			nStore := store.NewStore()
			for k, v := range input {
				val, _ := runCtx.GetValue(v)
				nStore.SetValue(k, val)
				OutWithDeep(deep, "create store:", k, val)
			}
			err := ep.Blocks[ep.NowOnPoint].Run(ctx, nStore, deep+1)
			if err != nil {
				errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkID, db.RunStatusError)
				if errChange != nil {
					return errChange
				}

				return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
			}
			out := ep.Blocks[ep.NowOnPoint].Outputs()
			for k, v := range out {
				val, _ := nStore.GetValue(k)
				OutWithDeep(deep, "update store:", k, val)
				ep.VarStore.SetValue(v, val)
			}

		} else {

			err := ep.Blocks[ep.NowOnPoint].Run(ctx, ep.VarStore,  deep+1)
			if err != nil {
				errChange := db.ChangeWorkStatus(ctx, ep.Storage, ep.WorkID, db.RunStatusError)
				if errChange != nil {
					return errChange
				}

				return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
			}

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
	out := ep.Outputs()
	for loc, glob := range out {
		val, _ := runCtx.GetValue(loc)
		runCtx.SetValue(glob, val)
		OutWithDeep(0, "writing:   ", loc, glob, val, ep.VarStore)
	}
	OutWithDeep(0, "varstore last:   ", ep.VarStore)
	return nil
}

func (ep *ExecutablePipeline) Next() string {
	return ep.NextStep
}

func (ep *ExecutablePipeline) CreateBlocks(c context.Context, source map[string]entity.EriusFunc) error {
	ep.Blocks = make(map[string]Runner)
	c, s := trace.StartSpan(c, "create_blocks")
	defer s.End()
	for k := range source {
		bn := k

		block := source[k]
		switch block.BlockType {
		case script.TypeInternal, "term":
			ep.Blocks[bn] = ep.CreateInternal(&block, bn)
		case "python3":
			fb := FunctionBlock{
				Name:           bn,
				FunctionName:   block.Title,
				FunctionInput:  make(map[string]string),
				FunctionOutput: make(map[string]string),
				NextStep:       block.Next,
				runURL:         ep.FaaS + "function/%s",
				//runURL: "https://openfaas.dev.autobp.mts.ru/function/%s.openfaas-fn",
			}

			for _, v := range block.Input {
				fb.FunctionInput[v.Name] = v.Global
			}

			for _, v := range block.Output {
				fb.FunctionOutput[v.Name] = v.Global
			}

			ep.Blocks[bn] = &fb
		case script.TypeScenario:
			p, err := db.GetExecutableByName(c, ep.Storage, block.Title)
			if err != nil {
				return err
			}

			epi := ExecutablePipeline{}
			epi.PipelineID = p.ID
			epi.VersionID = p.VersionID
			epi.Storage = ep.Storage
			epi.Entrypoint = p.Pipeline.Entrypoint
			epi.Logger = ep.Logger
			epi.FaaS = ep.FaaS
			epi.Input = make(map[string]string)
			epi.Output = make(map[string]string)
			epi.NextStep = block.Next
		 	err = epi.CreateWork(c, "Erius")
			if err != nil {
				return err
			}

			err = epi.CreateBlocks(c, p.Pipeline.Blocks)
			if err != nil {
				return err
			}

			for _, v := range block.Input {
				epi.Input[v.Name] = v.Global
				OutWithDeep( 0, "in:", v.Name, v.Global)
			}

			for _, v := range block.Output {
				epi.Output[v.Name] = v.Global
				OutWithDeep(0, "out:", v.Name, v.Global)
			}
			ep.Blocks[bn] = &epi
		}

	}

	return nil
}

func createInputBlock(title string, name, next string) *InputBlock {
	return &InputBlock{
		BlockName:     name,
		FunctionName:  title,
		NextStep:      next,
		FunctionInput: make(map[string]string),
	}
}

func createOutputBlock(title string, name, next string) *OutputBlock {
	return &OutputBlock{
		BlockName:      name,
		FunctionName:   title,
		NextStep:       next,
		FunctionOutput: make(map[string]string),
	}
}

func createIF(title string, name, onTrue, onFalse string) *IF {
	return &IF{
		Name:          name,
		FunctionName:  title,
		OnTrue:        onTrue,
		OnFalse:       onFalse,
		FunctionInput: make(map[string]string),
	}
}

func createStringsEqual(title string, name, onTrue, onFalse string) *StringsEqual {
	return &StringsEqual{
		Name:          name,
		FunctionName:  title,
		OnTrue:        onTrue,
		OnFalse:       onFalse,
		FunctionInput: make(map[string]string),
	}
}

func createConnectorBlock(title string, name, next string) *ConnectorBlock {
	return &ConnectorBlock{
		Name:           name,
		FunctionName:   title,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
		NextStep:       next,
	}
}

func createForBlock(title string, name, onTrue, onFalse string) *ForState {
	return &ForState{
		Name:           name,
		FunctionName:   title,
		OnTrue:         onTrue,
		OnFalse:        onFalse,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
	}
}

func (ep *ExecutablePipeline) CreateInternal(ef *entity.EriusFunc, name string) Runner {
	switch ef.Title {
	case "input":
		i := createInputBlock(ef.Title, name, ef.Next)

		for _, v := range ef.Output {
			i.FunctionInput[v.Name] = v.Global
		}

		return i
	case "output":
		i := createOutputBlock(ef.Title, name, ef.Next)
		for _, v := range ef.Output {
			i.FunctionOutput[v.Name] = v.Global
		}

		return i
	case "if":
		i := createIF(ef.Title, name, ef.OnTrue, ef.OnFalse)

		for _, v := range ef.Input {
			i.FunctionInput[v.Name] = v.Global
		}

		return i
	case "strings_is_equal":
		sie := createStringsEqual(ef.Title, name, ef.OnTrue, ef.OnFalse)

		for _, v := range ef.Input {
			sie.FunctionInput[v.Name] = v.Global
		}

		return sie
	case "connector":
		con := createConnectorBlock(ef.Title, name, ef.Next)

		for _, v := range ef.Input {
			con.FunctionInput[v.Name] = v.Global
		}

		for _, v := range ef.Output {
			con.FunctionOutput[v.Name] = v.Global
		}
		return con
	case "ngsa-send-alarm":
		ngsa := integration.NewNGSASendIntegration(ep.Storage, 3, name)
		for _, v := range ef.Input {
			ngsa.Input[v.Name] = v.Global
		}

		return ngsa
	case "for":
		f := createForBlock(ef.Title, name, ef.OnTrue, ef.OnFalse)
		for _, v := range ef.Input {
			f.FunctionInput[v.Name] = v.Global
		}
		for _, v := range ef.Output {
			f.FunctionOutput[v.Name] = v.Global
		}
		return f
	}

	return nil
}
