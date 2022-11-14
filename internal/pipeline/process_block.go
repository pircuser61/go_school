package pipeline

import (
	c "context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

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
	WorkTitle   string
	Initiator   string
	Storage     db.Database
	Sender      *mail.Service
	People      *people.Service
	ServiceDesc *servicedesc.Service
	FaaS        string
	VarStore    *store.VariableStore
	UpdateData  *script.BlockUpdateData
}

func (runCtx *BlockRunContext) Copy() *BlockRunContext {
	runCtxCopy := &(*runCtx)
	runCtxCopy.VarStore = &(*runCtx.VarStore)
	return runCtxCopy
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

func ProcessBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block")
	defer s.End()

	log := logger.GetLogger(ctx)

	defer func() {
		if err != nil {
			if changeErr := runCtx.changeTaskStatus(ctx, db.RunStatusError); changeErr != nil {
				log.WithError(changeErr).Error("couldn't change task status")
			}
		}
	}()

	status, getErr := runCtx.Storage.GetTaskStatus(ctx, runCtx.TaskID)
	if err != nil {
		err = getErr
		return
	}
	if status != db.RunStatusRunning && status != db.RunStatusCreated {
		return nil
	}

	if changeErr := runCtx.changeTaskStatus(ctx, db.RunStatusRunning); changeErr != nil {
		err = changeErr
		return
	}

	block, id, initErr := initBlock(ctx, name, bl, runCtx)
	if initErr != nil {
		err = initErr
		return
	}
	if (block.UpdateManual() && manual) || !block.UpdateManual() {
		err = updateBlock(ctx, block, name, id, runCtx)
		if err != nil {
			return
		}
	}
	if block.GetStatus() == StatusFinished || block.GetStatus() == StatusNoSuccess {
		activeBlocks, ok := block.Next(runCtx.VarStore)
		if !ok {
			err = runCtx.updateStepInDB(ctx, id, true, block.GetStatus())
			if err != nil {
				return
			}
			err = ErrCantGetNextStep
			return
		}
		for _, b := range activeBlocks {
			blockData, blockErr := runCtx.Storage.GetBlockDataFromVersion(ctx, runCtx.WorkNumber, b)
			if blockErr != nil {
				err = blockErr
				return
			}
			err = ProcessBlock(ctx, b, blockData, runCtx.Copy(), false)
			if err != nil {
				return
			}
		}
	}
	return nil
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
		return createGoWaitForAllInputsBlock(name, ef, runCtx)
	case BlockGoBeginParallelTaskId:
		return createGoStartParallelBlock(name, ef, runCtx), nil
	case BlockGoNotificationID:
		return createGoNotificationBlock(name, ef, runCtx)
	case BlockExecutableFunctionID:
		return createExecutableFunctionBlock(name, ef, runCtx)
	case BlockGoFormID:
		return createGoFormBlock(ctx, name, ef, runCtx)
	}

	return nil, errors.New("unknown go-block type: " + ef.TypeID)
}

func initBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, uuid.UUID, error) {
	block, err := CreateBlock(ctx, name, bl, runCtx)
	if err != nil {
		return nil, uuid.Nil, err
	}

	if _, ok := runCtx.VarStore.State[name]; !ok {
		state, stateErr := json.Marshal(block.GetState())
		if stateErr != nil {
			return nil, uuid.Nil, stateErr
		}
		runCtx.VarStore.ReplaceState(name, state)
	}

	id, _, err := runCtx.saveStepInDB(ctx, name, bl.TypeID)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return block, id, nil
}

func updateBlock(ctx c.Context, block Runner, name string, id uuid.UUID, runCtx *BlockRunContext) error {
	_, err := block.Update(ctx)
	if err != nil {
		key := name + KeyDelimiter + ErrorKey
		runCtx.VarStore.SetValue(key, err.Error())
	}
	err = runCtx.updateStepInDB(ctx, id, err != nil, block.GetStatus())
	if err != nil {
		return err
	}
	err = runCtx.updateStatusByStep(ctx, block.GetTaskHumanStatus())
	if err != nil {
		return err
	}
	return nil
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
