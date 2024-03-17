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
	name   string
	bl     *entity.EriusFunc
	runCtx *BlockRunContext
	manual bool
}

func NewBlockProcessor(name string, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) blockProcessor {
	return blockProcessor{
		name:   name,
		bl:     bl,
		runCtx: runCtx,
		manual: manual,
	}
}

//nolint:gocognit,gocyclo,nestif //it's ok
func (p *blockProcessor) ProcessBlock(ctx context.Context, its int) error {
	its++
	if its > 10 {
		return errors.New("took too long")
	}

	ctx, s := trace.StartSpan(ctx, "process_block")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("workNumber", p.runCtx.WorkNumber)

	status, getErr := p.runCtx.Services.Storage.GetTaskStatus(ctx, p.runCtx.TaskID)
	if getErr != nil {
		return p.handleError(ctx, log, getErr)
	}

	err := p.handleStatus(ctx, status)
	if err != nil {
		return p.handleError(ctx, log, err)
	}

	block, id, initErr := initBlock(ctx, p.name, p.bl, p.runCtx)
	if initErr != nil {
		return p.handleError(ctx, log, initErr)
	}

	isStatusFiniteBeforeUpdate := block.GetStatus() == StatusFinished ||
		block.GetStatus() == StatusNoSuccess ||
		block.GetStatus() == StatusError

	if (block.UpdateManual() && p.manual) || !block.UpdateManual() {
		if err = updateBlock(ctx, block, p.name, id, p.runCtx); err != nil {
			return p.handleError(ctx, log, err)
		}

		if p.bl.TypeID == "form" && p.runCtx.UpdateData != nil {
			activeBlocks, getActiveBlockErr := p.runCtx.Services.Storage.GetTaskActiveBlock(ctx, p.runCtx.TaskID.String(), p.name)
			if getActiveBlockErr != nil {
				return getActiveBlockErr
			}

			if processActiveErr := p.processActiveBlocks(ctx, activeBlocks, its, true); processActiveErr != nil {
				return processActiveErr
			}
		}
	}

	// handle edit form and other cases where we just poke the node
	if (p.runCtx.UpdateData != nil) && isStatusFiniteBeforeUpdate {
		return nil
	}

	taskHumanStatus, statusComment, action := block.GetTaskHumanStatus()

	err = p.runCtx.updateStatusByStep(ctx, taskHumanStatus, statusComment)
	if err != nil {
		return p.handleError(ctx, log, err)
	}

	newEvents := block.GetNewEvents()
	p.runCtx.BlockRunResults.NodeEvents = append(p.runCtx.BlockRunResults.NodeEvents, newEvents...)

	isArchived, err := p.runCtx.Services.Storage.CheckIsArchived(ctx, p.runCtx.TaskID)
	if err != nil {
		return p.handleError(ctx, log, err)
	}

	if isArchived || (block.GetStatus() != StatusFinished &&
		block.GetStatus() != StatusNoSuccess &&
		block.GetStatus() != StatusError) ||
		((p.runCtx.UpdateData != nil) && isStatusFiniteBeforeUpdate) {
		return nil
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
		return p.handleError(ctx, log, err)
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
			return p.handleError(ctx, log, err)
		}

		return p.handleError(ctx, log, ErrCantGetNextStep)
	}

	err = p.processActiveBlocks(ctx, activeBlocks, its, false)
	if err != nil {
		return p.handleError(ctx, log, err)
	}

	return nil
}

func (p *blockProcessor) handleError(ctx context.Context, log logger.Logger, err error) error {
	if err != nil && !errors.Is(err, UserIsNotPartOfProcessErr{}) {
		log.WithError(err).Error("couldn't process block")

		changeErr := p.runCtx.updateTaskStatus(ctx, db.RunStatusError, "", db.SystemLogin)
		if changeErr != nil {
			log.WithError(changeErr).Error("couldn't change task status")
		}
	}

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

func (p *blockProcessor) processActiveBlocks(ctx context.Context, activeBlocks []string, its int, updateVarStore bool) error {
	for _, blockName := range activeBlocks {
		blockData, blockErr := p.runCtx.Services.Storage.GetBlockDataFromVersion(ctx, p.runCtx.WorkNumber, blockName)
		if blockErr != nil {
			return blockErr
		}

		ctxCopy := p.runCtx.Copy()

		if updateVarStore {
			ctxCopy.UpdateData = &script.BlockUpdateData{Action: string(entity.TaskUpdateActionReload)}

			storage, getErrVarStorage := p.runCtx.Services.Storage.GetVariableStorageForStep(ctx, p.runCtx.TaskID, blockName)
			if getErrVarStorage != nil {
				return getErrVarStorage
			}

			ctxCopy.VarStore = storage
		}

		processor := NewBlockProcessor(blockName, blockData, ctxCopy, updateVarStore)

		err := processor.ProcessBlock(ctx, its)
		if err != nil {
			return err
		}

		p.runCtx.BlockRunResults.NodeEvents = append(p.runCtx.BlockRunResults.NodeEvents, ctxCopy.BlockRunResults.NodeEvents...)
	}

	return nil
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
