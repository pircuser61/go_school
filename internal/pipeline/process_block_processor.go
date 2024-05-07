package pipeline

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

var ErrCantGetNextStep = errors.New("can't get next step")

type blockProcessor struct {
	name    string
	bl      *entity.EriusFunc
	runCtx  *BlockRunContext
	storage db.Database
	manual  bool
}

func newBlockProcessor(name string, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) blockProcessor {
	return blockProcessor{
		name:   name,
		bl:     bl,
		runCtx: runCtx,
		manual: manual,
	}
}

//nolint:gocognit,gocyclo,nestif //it's ok
func (p *blockProcessor) ProcessBlock(ctx context.Context, its int) (string, error) {
	log := logger.GetLogger(ctx).
		WithField("funcName", "ProcessBlock").
		WithField("pipelineID", p.runCtx.PipelineID).
		WithField("versionID", p.runCtx.VersionID).
		WithField("workNumber", p.runCtx.WorkNumber).
		WithField("workID", p.runCtx.TaskID).
		WithField("clientID", p.runCtx.ClientID).
		WithField("stepName", p.name).
		WithField("stepID", "")
	ctx = logger.WithLogger(ctx, log)

	its++
	if its > 10 {
		log.Error("took too long")

		return p.name, errors.New("took too long")
	}

	ctx, s := trace.StartSpan(ctx, "process_block")
	defer s.End()

	log = logger.GetLogger(ctx).WithField("workNumber", p.runCtx.WorkNumber)

	err := p.startTx(ctx)
	if err != nil {
		log.WithError(err).Error("failed start tx")

		return p.name, err
	}

	status, getErr := p.runCtx.Services.Storage.GetTaskStatus(ctx, p.runCtx.TaskID)
	if getErr != nil {
		return p.name, p.handleErrorWithRollback(ctx, log, getErr)
	}

	err = p.handleStatus(ctx, status)
	if err != nil {
		return p.name, p.handleErrorWithRollback(ctx, log, err)
	}

	block, id, initErr := initBlock(ctx, p.name, p.bl, p.runCtx)
	if initErr != nil {
		log = log.WithField("stepID", id)

		return p.name, p.handleErrorWithRollback(ctx, log, initErr)
	}

	log = log.WithField("stepID", id)
	ctx = logger.WithLogger(ctx, log)

	if block == nil {
		err = p.commitTx(ctx)
		if err != nil {
			log.WithError(err).Error("couldn't commit tx")
		}

		return p.name, nil
	}

	isStatusFiniteBeforeUpdate := (block.GetStatus() == StatusFinished ||
		block.GetStatus() == StatusNoSuccess ||
		block.GetStatus() == StatusError) &&
		(p.runCtx.UpdateData != nil && p.runCtx.UpdateData.Action != string(entity.TaskUpdateActionReload))

	if (block.UpdateManual() && p.manual) || !block.UpdateManual() {
		if err = updateBlock(ctx, block, p.name, id, p.runCtx); err != nil {
			return p.name, p.handleErrorWithRollback(ctx, log, err)
		}

		refillForm := p.bl.TypeID == "form" && p.runCtx.UpdateData != nil &&
			p.runCtx.UpdateData.Action == string(entity.TaskUpdateActionRequestFillForm)
		if refillForm && isStatusFiniteBeforeUpdate {
			activeBlocks, getActiveBlockErr := p.runCtx.Services.Storage.GetTaskActiveBlock(ctx, p.runCtx.TaskID.String(), p.name)
			if getActiveBlockErr != nil {
				return p.name, p.handleErrorWithRollback(ctx, log, getActiveBlockErr)
			}

			// эта функция уже будет обрабатывать ошибку, ошибку которую она вернула не нужно обрабатывать повторно
			failedBlock, processActiveErr := p.processActiveBlocks(ctx, activeBlocks, its, true)
			if processActiveErr != nil {
				return failedBlock, processActiveErr
			}

			return failedBlock, nil
		}
	}

	// handle edit form and other cases where we just poke the node
	if isStatusFiniteBeforeUpdate {
		err = p.commitTx(ctx)
		if err != nil {
			log.WithError(err).Error("couldn't commit tx")
		}

		return p.name, nil
	}

	taskHumanStatus, statusComment, action := block.GetTaskHumanStatus()

	err = p.runCtx.updateStatusByStep(ctx, taskHumanStatus, statusComment)
	if err != nil {
		return p.name, p.handleErrorWithRollback(ctx, log, err)
	}

	newEvents := block.GetNewEvents()
	p.runCtx.BlockRunResults.NodeEvents = append(p.runCtx.BlockRunResults.NodeEvents, newEvents...)

	newKafkaEvents := block.GetNewKafkaEvents()
	p.runCtx.BlockRunResults.NodeKafkaEvents = append(p.runCtx.BlockRunResults.NodeKafkaEvents, newKafkaEvents...)

	isArchived, err := p.runCtx.Services.Storage.CheckIsArchived(ctx, p.runCtx.TaskID)
	if err != nil {
		return p.name, p.handleErrorWithRollback(ctx, log, err)
	}

	if isArchived || (block.GetStatus() != StatusFinished &&
		block.GetStatus() != StatusNoSuccess &&
		block.GetStatus() != StatusError) || isStatusFiniteBeforeUpdate {
		err = p.commitTx(ctx)
		if err != nil {
			log.WithError(err).Error("couldn't commit tx")
		}

		return p.name, nil
	}

	err = p.runCtx.handleInitiatorNotify(
		ctx,
		handleInitiatorNotifyParams{
			step:     p.name,
			stepType: p.bl.TypeID,
			action:   action,
			status:   taskHumanStatus,
		},
	)
	if err != nil {
		return p.name, p.handleErrorWithRollback(ctx, log, err)
	}

	activeBlocks, ok := block.Next(p.runCtx.VarStore)
	if !ok {
		err = p.runCtx.updateStepInDB(ctx, &updateStepDTO{
			id:              id,
			name:            p.name,
			status:          block.GetStatus(),
			hasError:        true,
			members:         block.Members(),
			deadlines:       []Deadline{},
			attachments:     block.BlockAttachments(),
			currentExecutor: CurrentExecutorData{},
		})
		if err != nil {
			return p.name, p.handleErrorWithRollback(ctx, log, err)
		}

		return p.name, p.handleErrorWithCommit(ctx, log, ErrCantGetNextStep)
	}

	// эта функция уже будет обрабатывать ошибку, ошибку которую она вернула не нужно обрабатывать повторно
	failedBlock, err := p.processActiveBlocks(ctx, activeBlocks, its, false)
	if err != nil {
		return failedBlock, err
	}

	return "", nil
}

func (p *blockProcessor) handleErrorWithCommit(ctx context.Context, log logger.Logger, err error) error {
	if err != nil && !errors.Is(err, UserIsNotPartOfProcessErr{}) {
		log.WithError(err).Error("couldn't process block")
	}

	commitErr := p.commitTx(ctx)
	if commitErr != nil {
		log.WithError(commitErr).Error("couldn't commit tx")
	}

	return err
}

func (p *blockProcessor) handleErrorWithRollback(ctx context.Context, log logger.Logger, err error) error {
	rollbackErr := p.rollbackTx(ctx)
	if rollbackErr != nil {
		log.WithError(rollbackErr).Error("couldn't rollback tx")
	}

	if err != nil && !errors.Is(err, UserIsNotPartOfProcessErr{}) {
		log.WithError(err).Error("couldn't process block")
	}

	return err
}

func (p *blockProcessor) startTx(ctx context.Context) error {
	txStorage, err := p.runCtx.Services.Storage.StartTransaction(ctx)
	if err != nil {
		return err
	}

	p.storage = p.runCtx.Services.Storage
	p.runCtx.Services.Storage = txStorage

	return nil
}

func (p *blockProcessor) commitTx(ctx context.Context) error {
	err := p.runCtx.Services.Storage.CommitTransaction(ctx)
	p.runCtx.Services.Storage = p.storage

	return err
}

func (p *blockProcessor) rollbackTx(ctx context.Context) error {
	err := p.runCtx.Services.Storage.RollbackTransaction(ctx)
	p.runCtx.Services.Storage = p.storage

	return err
}

//nolint:unparam //мб когда нибудь comment будет вызываться не с пустым значением
func (runCtx *BlockRunContext) updateTaskStatus(ctx context.Context, taskStatus int, comment, author string) error {
	errChange := runCtx.Services.Storage.UpdateTaskStatus(ctx, runCtx.TaskID, taskStatus, comment, author)
	if errChange != nil {
		runCtx.VarStore.AddError(errChange)

		return errChange
	}

	return nil
}

func (runCtx *BlockRunContext) updateStatusByStep(ctx context.Context, status TaskHumanStatus, statusComment string) error {
	if status == "" {
		return nil
	}

	_, err := runCtx.Services.Storage.UpdateTaskHumanStatus(ctx, runCtx.TaskID, string(status), statusComment)

	return err
}

// эта функция уже будет обрабатывать ошибку, ошибку которую она вернула не нужно обрабатывать повторно
func (p *blockProcessor) processActiveBlocks(ctx context.Context, activeBlocks []string, its int, updateVarStore bool) (string, error) {
	log := logger.GetLogger(ctx).WithField("funcName", "processActiveBlocks")

	for _, blockName := range activeBlocks {
		blockData, blockErr := p.runCtx.Services.Storage.GetStepDataFromVersion(ctx, p.runCtx.WorkNumber, blockName)
		if blockErr != nil {
			return p.name, p.handleErrorWithRollback(ctx, log, blockErr)
		}

		tmpCtx := p.runCtx.Copy()

		err := CreateBlockInDB(ctx, blockName, blockData.TypeID, tmpCtx)
		if err != nil {
			return p.name, p.handleErrorWithRollback(ctx, log, err)
		}
	}

	err := p.commitTx(ctx)
	if err != nil {
		log.WithError(err).Error("could`t commit tx")

		return p.name, err
	}

	for _, blockName := range activeBlocks {
		blockData, blockErr := p.runCtx.Services.Storage.GetStepDataFromVersion(ctx, p.runCtx.WorkNumber, blockName)
		if blockErr != nil {
			return blockName, p.handleErrorWithRollback(ctx, log, blockErr)
		}

		ctxCopy := p.runCtx.Copy()

		if updateVarStore {
			ctxCopy.UpdateData = &script.BlockUpdateData{Action: string(entity.TaskUpdateActionReload)}

			storage, getErrVarStorage := p.runCtx.Services.Storage.GetVariableStorageForStep(ctx, p.runCtx.TaskID, blockName)
			if getErrVarStorage != nil {
				return blockName, p.handleErrorWithRollback(ctx, log, getErrVarStorage)
			}

			ctxCopy.VarStore = storage
		}

		ctxCopy.Services.Storage = p.storage

		processor := newBlockProcessor(blockName, blockData, ctxCopy, updateVarStore)

		failedBlock, err := processor.ProcessBlock(ctx, its)
		if err != nil {
			return failedBlock, err
		}

		p.runCtx.BlockRunResults.NodeEvents = append(p.runCtx.BlockRunResults.NodeEvents, processor.runCtx.BlockRunResults.NodeEvents...)
		p.runCtx.BlockRunResults.NodeKafkaEvents = append(
			p.runCtx.BlockRunResults.NodeKafkaEvents,
			processor.runCtx.BlockRunResults.NodeKafkaEvents...,
		)
	}

	return "", nil
}

func (p *blockProcessor) updateTaskExecDeadline(ctx context.Context) error {
	sc, err := p.runCtx.Services.Storage.GetVersionByWorkNumber(ctx, p.runCtx.WorkNumber)
	if err != nil {
		return err
	}
	// compute deadline using sla from process version settings
	versionSettings, errSLA := p.runCtx.Services.Storage.GetSLAVersionSettings(ctx, sc.VersionID.String())
	if errSLA != nil {
		return errSLA
	}

	times, timesErr := p.runCtx.Services.Storage.GetTaskInWorkTime(ctx, p.runCtx.WorkNumber)
	if timesErr != nil {
		return timesErr
	}

	slaInfoPtr, getSLAInfoErr := p.runCtx.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{
			{
				StartedAt:  times.StartedAt,
				FinishedAt: times.StartedAt.Add(time.Hour * 24 * 100),
			},
		},
		WorkType: sla.WorkHourType(versionSettings.WorkType),
	})
	if getSLAInfoErr != nil {
		return getSLAInfoErr
	}

	deadline := p.runCtx.Services.SLAService.ComputeMaxDate(
		times.StartedAt,
		float32(versionSettings.SLA),
		slaInfoPtr)

	return p.runCtx.Services.Storage.SetExecDeadline(ctx, p.runCtx.TaskID.String(), deadline)
}

func (p *blockProcessor) handleStatus(ctx context.Context, status int) error {
	switch status {
	case db.RunStatusCreated:
		if changeErr := p.runCtx.updateTaskStatus(ctx, db.RunStatusRunning, "", db.SystemLogin); changeErr != nil {
			return changeErr
		}
	case db.RunStatusRunning:
	case db.RunStatusCanceled:
		return errors.New("couldn't process canceled block")
	}

	return nil
}
