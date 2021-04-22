package pipeline

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/integration"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

var errUnknownBlock = errors.New("unknown block")

type ExecutablePipeline struct {
	TaskID        uuid.UUID
	PipelineID    uuid.UUID
	VersionID     uuid.UUID
	Storage       db.Database
	EntryPoint    string
	NowOnPoint    string
	VarStore      *store.VariableStore
	Blocks        map[string]Runner
	NextStep      string
	Input         map[string]string
	Output        map[string]string
	Name          string
	PipelineModel *entity.EriusScenario
	HTTPClient    *http.Client
	Remedy        string

	FaaS string
}

func (ep *ExecutablePipeline) Inputs() map[string]string {
	return ep.Input
}

func (ep *ExecutablePipeline) Outputs() map[string]string {
	return ep.Output
}

func (ep *ExecutablePipeline) IsScenario() bool {
	return true
}

func (ep *ExecutablePipeline) CreateTask(ctx context.Context, author string, isDebugMode bool, parameters []byte) error {
	ep.TaskID = uuid.New()

	_, err := ep.Storage.CreateTask(ctx, ep.TaskID, ep.VersionID, author, isDebugMode, parameters)
	if err != nil {
		return err
	}

	return nil
}

func (ep *ExecutablePipeline) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return ep.DebugRun(ctx, runCtx)
}

func (ep *ExecutablePipeline) saveStep(ctx context.Context, hasError bool) error {
	storageData, errSerialize := json.Marshal(ep.VarStore)
	if errSerialize != nil {
		return errSerialize
	}

	breakPoints := ep.VarStore.StopPoints.BreakPointsList()

	errSaveStep := ep.Storage.SaveStepContext(ctx, ep.TaskID, ep.NowOnPoint, storageData, breakPoints, hasError)
	if errSaveStep != nil {
		return errSaveStep
	}

	return nil
}

func (ep *ExecutablePipeline) finallyError(ctx context.Context, err error) error {
	ep.VarStore.AddError(err)

	errChange := ep.changeTaskStatus(ctx, db.RunStatusError)
	if errChange != nil {
		return errChange
	}

	errSaveStep := ep.saveStep(ctx, true)
	if errSaveStep != nil {
		return errSaveStep
	}

	return err
}

func (ep *ExecutablePipeline) changeTaskStatus(ctx context.Context, taskStatus int) error {
	errChange := ep.Storage.ChangeTaskStatus(ctx, ep.TaskID, taskStatus)
	if errChange != nil {
		ep.VarStore.AddError(errChange)

		return errChange
	}

	return nil
}

//nolint:gocognit,gocyclo //its really complex
func (ep *ExecutablePipeline) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "pipeline_flow")
	defer s.End()

	log := logger.GetLogger(ctx)

	ep.VarStore = runCtx

	if ep.NowOnPoint == "" {
		ep.NowOnPoint = ep.EntryPoint
	}

	errChange := ep.Storage.ChangeTaskStatus(ctx, ep.TaskID, db.RunStatusRunning)
	if errChange != nil {
		return errChange
	}

	for ep.NowOnPoint != "" {
		log.Info("executing", ep.NowOnPoint)

		now, ok := ep.Blocks[ep.NowOnPoint]
		if !ok {
			return ep.finallyError(ctx, errUnknownBlock)
		}

		//nolint:nestif //its really complexive
		if now.IsScenario() {
			ep.VarStore.AddStep(ep.NowOnPoint)

			nStore := store.NewStore()

			input := ep.Blocks[ep.NowOnPoint].Inputs()
			for local, global := range input {
				val, _ := runCtx.GetValue(global)
				nStore.SetValue(local, val)
			}

			err := ep.Blocks[ep.NowOnPoint].DebugRun(ctx, nStore)
			if err != nil {
				_ = ep.finallyError(ctx, err)
				return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
			}

			out := ep.Blocks[ep.NowOnPoint].Outputs()
			for inner, outer := range out {
				val, _ := nStore.GetValue(inner)
				ep.VarStore.SetValue(outer, val)
			}
		} else {
			err := ep.Blocks[ep.NowOnPoint].DebugRun(ctx, ep.VarStore)
			if err != nil {
				_ = ep.finallyError(ctx, err)

				return errors.Errorf("error while executing pipeline on step %s: %s", ep.NowOnPoint, err.Error())
			}
		}

		_, hasError := runCtx.GetValue(ep.NowOnPoint + KeyDelimiter + ErrorKey)

		errSaveStep := ep.saveStep(ctx, hasError)
		if errSaveStep != nil {
			return ep.finallyError(ctx, errSaveStep)
		}

		ep.NowOnPoint, ok = ep.Blocks[ep.NowOnPoint].Next(ep.VarStore)
		if !ok {
			return ep.finallyError(ctx, ErrCantGetNextStep)
		}

		if runCtx.StopPoints.IsStopPoint(ep.NowOnPoint) {
			errChangeStopped := ep.changeTaskStatus(ctx, db.RunStatusStopped)
			if errChangeStopped != nil {
				return errChange
			}

			return nil
		}
	}

	errChangeFinished := ep.changeTaskStatus(ctx, db.RunStatusFinished)
	if errChangeFinished != nil {
		return errChange
	}

	for _, glob := range ep.PipelineModel.Output {
		val, _ := runCtx.GetValue(glob.Global)
		runCtx.SetValue(glob.Name, val)
	}

	return nil
}

func (ep *ExecutablePipeline) Next(*store.VariableStore) (string, bool) {
	return ep.NextStep, true
}

func (ep *ExecutablePipeline) NextSteps() []string {
	return []string{ep.NextStep}
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
		case script.TypePython3:
			fb := FunctionBlock{
				Name:           bn,
				FunctionName:   block.Title,
				FunctionInput:  make(map[string]string),
				FunctionOutput: make(map[string]string),
				NextStep:       block.Next,
				RunURL:         ep.FaaS + "function/%s",
			}

			for _, v := range block.Input {
				fb.FunctionInput[v.Name] = v.Global
			}

			for _, v := range block.Output {
				fb.FunctionOutput[v.Name] = v.Global
			}

			ep.Blocks[bn] = &fb
		case script.TypeScenario:
			p, err := ep.Storage.GetExecutableByName(c, block.Title)
			if err != nil {
				return err
			}

			epi := ExecutablePipeline{}
			epi.PipelineID = p.ID
			epi.VersionID = p.VersionID
			epi.Storage = ep.Storage
			epi.EntryPoint = p.Pipeline.Entrypoint
			epi.FaaS = ep.FaaS
			epi.Input = make(map[string]string)
			epi.Output = make(map[string]string)
			epi.NextStep = block.Next
			epi.Name = block.Title
			epi.PipelineModel = p

			parametersMap := make(map[string]interface{})
			for _, v := range block.Input {
				parametersMap[v.Name] = v.Global
			}

			parameters, err := json.Marshal(parametersMap)
			if err != nil {
				return err
			}

			err = epi.CreateTask(c, "Erius", false, parameters)
			if err != nil {
				return err
			}

			err = epi.CreateBlocks(c, p.Pipeline.Blocks)
			if err != nil {
				return err
			}

			for _, v := range block.Input {
				epi.Input[p.Name+KeyDelimiter+v.Name] = v.Global
			}

			for _, v := range block.Output {
				epi.Output[v.Name] = v.Global
			}

			ep.Blocks[bn] = &epi
		}
	}

	return nil
}

func createIF(title, name, onTrue, onFalse string) *IF {
	return &IF{
		Name:          name,
		FunctionName:  title,
		OnTrue:        onTrue,
		OnFalse:       onFalse,
		FunctionInput: make(map[string]string),
	}
}

func createStringsEqual(title, name, onTrue, onFalse string) *StringsEqual {
	return &StringsEqual{
		Name:          name,
		FunctionName:  title,
		OnTrue:        onTrue,
		OnFalse:       onFalse,
		FunctionInput: make(map[string]string),
	}
}

func createConnectorBlock(title, name, next string) *ConnectorBlock {
	return &ConnectorBlock{
		Name:           name,
		FunctionName:   title,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
		NextStep:       next,
	}
}

func createForBlock(title, name, onTrue, onFalse string) *ForState {
	return &ForState{
		Name:           name,
		FunctionName:   title,
		OnTrue:         onTrue,
		OnFalse:        onFalse,
		FunctionInput:  make(map[string]string),
		FunctionOutput: make(map[string]string),
	}
}

//nolint:gocyclo //need bigger cyclomatic
func (ep *ExecutablePipeline) CreateInternal(ef *entity.EriusFunc, name string) Runner {
	switch ef.Title {
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
		ngsa := integration.NewNGSASendIntegration(ep.Storage)
		for _, v := range ef.Input {
			ngsa.Input[v.Name] = v.Global
		}

		ngsa.Name = ef.Title
		ngsa.NextBlock = ef.Next

		return ngsa
	case "remedy-send-createmi":
		rem := integration.NewRemedySendCreateMI(ep.Remedy, ep.HTTPClient)
		for _, v := range ef.Input {
			rem.Input[v.Name] = v.Global
		}

		rem.Name = ef.Title
		rem.NextBlock = ef.Next

		return rem
	case "remedy-send-createproblem":
		rem := integration.NewRemedySendCreateProblem(ep.Remedy, ep.HTTPClient)
		for _, v := range ef.Input {
			rem.Input[v.Name] = v.Global
		}

		rem.Name = ef.Title
		rem.NextBlock = ef.Next

		return rem
	case "remedy-send-creatework":
		rem := integration.NewRemedySendCreateWork(ep.Remedy, ep.HTTPClient)
		for _, v := range ef.Input {
			rem.Input[v.Name] = v.Global
		}

		rem.Name = ef.Title
		rem.NextBlock = ef.Next

		return rem
	case "remedy-send-updatemi":
		rem := integration.NewRemedySendUpdateMI(ep.Remedy, ep.HTTPClient)
		for _, v := range ef.Input {
			rem.Input[v.Name] = v.Global
		}

		rem.Name = ef.Title
		rem.NextBlock = ef.Next

		return rem
	case "remedy-send-updateproblem":
		rem := integration.NewRemedySendUpdateProblem(ep.Remedy, ep.HTTPClient)
		for _, v := range ef.Input {
			rem.Input[v.Name] = v.Global
		}

		rem.Name = ef.Title
		rem.NextBlock = ef.Next

		return rem
	case "remedy-send-updatework":
		rem := integration.NewRemedySendUpdateWork(ep.Remedy, ep.HTTPClient)
		for _, v := range ef.Input {
			rem.Input[v.Name] = v.Global
		}

		rem.Name = ef.Title
		rem.NextBlock = ef.Next

		return rem
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
