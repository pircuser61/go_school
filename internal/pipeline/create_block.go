package pipeline

import (
	c "context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type BlockRunContext struct {
	TaskID      uuid.UUID
	WorkNumber  string
	Initiator   string
	Storage     db.Database
	Sender      *mail.Service
	People      *people.Service
	ServiceDesc *servicedesc.Service
	FaaS        string
	VarStore    *store.VariableStore
}

func (runCtx *BlockRunContext) saveStepInDB(ctx c.Context, name, stepType string) (uuid.UUID, time.Time, error) {
	storageData, errSerialize := json.Marshal(runCtx.VarStore)
	if errSerialize != nil {
		return db.NullUuid, time.Time{}, errSerialize
	}

	return runCtx.Storage.SaveStepContext(ctx, &db.SaveStepRequest{
		WorkID:      runCtx.TaskID,
		StepType:    stepType,
		StepName:    name,
		Content:     storageData,
		BreakPoints: []string{},
		HasError:    false,
		Status:      string(StatusNew),
	})
}

func (runCtx *BlockRunContext) updateStepInDB(ctx c.Context, id uuid.UUID, hasError bool, status Status) error {
	storageData, err := json.Marshal(runCtx.VarStore)
	if err != nil {
		return err
	}

	return runCtx.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:             id,
		Content:        storageData,
		BreakPoints:    []string{},
		HasError:       hasError,
		Status:         string(status),
		WithoutContent: status != StatusFinished && status != StatusCancel && status != StatusNoSuccess,
	})
}

func CreateAndSaveBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, error) {
	ctx, s := trace.StartSpan(ctx, "create_and_save_block")
	defer s.End()

	_, _, err := runCtx.saveStepInDB(ctx, name, bl.BlockType)
	if err != nil {
		return nil, err
	}
	block, err := CreateBlock(ctx, name, bl, runCtx)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func CreateBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, error) {
	ctx, s := trace.StartSpan(ctx, "create_block")
	defer s.End()

	switch bl.BlockType {
	case script.TypeGo:
		return createGoBlock(ctx, bl, name, runCtx)
	case script.TypeExternal:
		return createExecutableFunctionBlock(name, bl, runCtx)
	case script.TypeScenario:
		p, err := runCtx.Storage.GetExecutableByName(ctx, bl.Title)
		if err != nil {
			return nil, err
		}

		epi := ExecutablePipeline{}
		epi.PipelineID = p.ID
		epi.VersionID = p.VersionID
		epi.Storage = runCtx.Storage
		epi.EntryPoint = p.Pipeline.Entrypoint
		epi.FaaS = runCtx.FaaS
		epi.Input = make(map[string]string)
		epi.Output = make(map[string]string)
		epi.Nexts = bl.Next
		epi.Name = bl.Title
		epi.PipelineModel = p
		epi.RunContext = runCtx

		parametersMap := make(map[string]interface{})
		for _, v := range bl.Input {
			parametersMap[v.Name] = v.Global
		}

		parameters, err := json.Marshal(parametersMap)
		if err != nil {
			return nil, err
		}

		err = epi.CreateTask(ctx, &CreateTaskDTO{
			Author:  "Erius",
			IsDebug: false,
			Params:  parameters,
		})
		if err != nil {
			return nil, err
		}

		err = epi.CreateBlocks(ctx, p.Pipeline.Blocks)
		if err != nil {
			return nil, err
		}

		for _, v := range bl.Input {
			epi.Input[p.Name+KeyDelimiter+v.Name] = v.Global
		}

		for _, v := range bl.Output {
			epi.Output[v.Name] = v.Global
		}

		return &epi, nil
	}

	return nil, errors.Errorf("can't create block with type: %s", bl.BlockType)
}

func createGoBlock(ctx c.Context, ef *entity.EriusFunc, name string, runCtx *BlockRunContext) (Runner, error) {
	switch ef.TypeID {
	case BlockGoIfID:
		return createGoIfBlock(name, ef, runCtx)
	case BlockGoTestID:
		return createGoTestBlock(name, ef, runCtx), nil
	case BlockGoApproverID:
		return createGoApproverBlock(ctx, name, ef, runCtx)
	case BlockGoSdApplicationID:
		return createGoSdApplicationBlock(name, ef, runCtx)
	case BlockGoExecutionID:
		return createGoExecutionBlock(ctx, name, ef, runCtx)
	case BlockGoStartId:
		return createGoStartBlock(name, ef, runCtx), nil
	case BlockGoEndId:
		return createGoEndBlock(name, ef, runCtx), nil
	case BlockWaitForAllInputsId:
		return createGoWaitForAllInputsBlock(name, ef, runCtx), nil
	case BlockGoBeginParallelTaskId:
		return createGoStartParallelBlock(name, ef, runCtx), nil
	case BlockGoNotificationID:
		return createGoNotificationBlock(name, ef, runCtx)
	case BlockExecutableFunctionID:
		return createExecutableFunctionBlock(name, ef, runCtx)
	case BlockGoFormID:
		return createGoFormBlock(name, ef, runCtx)
	}

	return nil, errors.New("unknown go-block type: " + ef.TypeID)
}
