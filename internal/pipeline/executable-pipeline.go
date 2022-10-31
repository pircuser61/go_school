package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
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

type void = struct{}

const (
	// TODO maybe there is a better way to save work id in variable store
	keyStepWorkId = "work_id"
)

var errUnknownBlock = errors.New("unknown block")

type ExecutablePipeline struct {
	TaskID        uuid.UUID
	WorkNumber    string
	PipelineID    uuid.UUID
	VersionID     uuid.UUID
	Storage       db.Database
	EntryPoint    string
	StepType      string
	ActiveBlocks  map[string]void
	SkippedBlocks map[string]void
	VarStore      *store.VariableStore
	Blocks        map[string]Runner
	Nexts         map[string][]string
	Sockets       []script.Socket
	Input         map[string]string
	Output        map[string]string
	Name          string
	PipelineModel *entity.EriusScenario
	HTTPClient    *http.Client
	Remedy        string
	Sender        *mail.Service
	People        *people.Service
	ServiceDesc   *servicedesc.Service

	FaaS string

	endExecution bool

	Initiator              string
	initiatorEmail         string
	currDescription        string
	notifiedBlocks         map[string][]TaskHumanStatus
	prevUpdateStatusBlocks map[string]TaskHumanStatus
}

func (ep *ExecutablePipeline) GetStatus() Status {
	switch {
	case ep.IsOver():
		return StatusFinished
	case ep.ReadyToStart():
		return StatusReady
	case len(ep.ActiveBlocks) != 0:
		return StatusRunning
	default:
		return StatusIdle
	}
}

func (ep *ExecutablePipeline) IsOver() bool {
	return len(ep.ActiveBlocks) == 0 || ep.endExecution
}

func (ep *ExecutablePipeline) MergeActiveBlocks(blocks []string) {
	for _, block := range blocks {
		_, exist := ep.ActiveBlocks[block]
		if !exist {
			ep.ActiveBlocks[block] = void{}
		}
	}
}

func (ep *ExecutablePipeline) MergeSkippedBlocks(blocks []string) {
	for _, block := range blocks {
		_, exist := ep.SkippedBlocks[block]
		if !exist {
			ep.SkippedBlocks[block] = void{}
		}
	}
}

func (ep *ExecutablePipeline) ReadyToStart() bool {
	return len(ep.ActiveBlocks) == 0 && ep.EntryPoint == BlockGoFirstStart
}

func (ep *ExecutablePipeline) GetTaskHumanStatus() TaskHumanStatus {
	// TODO: проверять, что нет ошибок (потому что только тогда мы Done)
	if len(ep.ActiveBlocks) == 0 {
		if ep.endExecution {
			return "" // не обновляем статус т.к. блок, завершившийся неуспешно, сам проставляет статус
		}
		return StatusDone
	}
	return StatusNew
}

func (ep *ExecutablePipeline) GetType() string {
	return BlockScenario
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

type CreateTaskDTO struct {
	Author     string
	IsDebug    bool
	Params     []byte
	WorkNumber string
}

func (ep *ExecutablePipeline) CreateTask(ctx c.Context, dto *CreateTaskDTO) error {
	ep.TaskID = uuid.New()

	task, err := ep.Storage.CreateTask(ctx, &db.CreateTaskDTO{
		TaskID:     ep.TaskID,
		VersionID:  ep.VersionID,
		Author:     dto.Author,
		WorkNumber: dto.WorkNumber,
		IsDebug:    dto.IsDebug,
		Params:     dto.Params,
	})
	if err != nil {
		return err
	}

	ep.WorkNumber = task.WorkNumber
	return nil
}

func (ep *ExecutablePipeline) Run(ctx c.Context, runCtx *store.VariableStore) error {
	return ep.DebugRun(ctx, nil, runCtx)
}

func (ep *ExecutablePipeline) createStep(ctx c.Context, name string, hasError bool, status Status) (uuid.UUID, time.Time, error) {
	storageData, errSerialize := json.Marshal(ep.VarStore)
	if errSerialize != nil {
		return db.NullUuid, time.Time{}, errSerialize
	}

	breakPoints := ep.VarStore.StopPoints.BreakPointsList()

	return ep.Storage.SaveStepContext(ctx, &db.SaveStepRequest{
		WorkID:      ep.TaskID,
		StepType:    ep.StepType,
		StepName:    name,
		Content:     storageData,
		BreakPoints: breakPoints,
		HasError:    hasError,
		Status:      string(status),
	})
}

func (ep *ExecutablePipeline) updateStep(ctx c.Context, id uuid.UUID, hasError bool, status Status) error {
	storageData, err := json.Marshal(ep.VarStore)
	if err != nil {
		return err
	}

	breakPoints := ep.VarStore.StopPoints.BreakPointsList()

	return ep.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:             id,
		Content:        storageData,
		BreakPoints:    breakPoints,
		HasError:       hasError,
		Status:         string(status),
		WithoutContent: status != StatusFinished && status != StatusCancel && status != StatusNoSuccess,
	})
}

func (ep *ExecutablePipeline) changeTaskStatus(ctx c.Context, taskStatus int) error {
	errChange := ep.Storage.ChangeTaskStatus(ctx, ep.TaskID, taskStatus)
	if errChange != nil {
		ep.VarStore.AddError(errChange)

		return errChange
	}

	return nil
}

// TODO: что-то сделать
func (ep *ExecutablePipeline) updateStatusByStep(ctx c.Context, step string, status TaskHumanStatus) error {
	if ep.prevUpdateStatusBlocks == nil {
		ep.prevUpdateStatusBlocks = make(map[string]TaskHumanStatus)
	}
	prev := ep.prevUpdateStatusBlocks[step]
	if prev == status {
		return nil
	}
	if status != "" {
		if err := ep.Storage.UpdateTaskHumanStatus(ctx, ep.TaskID, string(status)); err != nil {
			return err
		}
	}
	ep.prevUpdateStatusBlocks[step] = status
	return nil
}

func (ep *ExecutablePipeline) dumpTaskBlocksData(ctx c.Context) error {
	notifiedBlocks := make(map[string][]string)
	for i := range ep.notifiedBlocks {
		for j := range ep.notifiedBlocks[i] {
			notifiedBlocks[i] = append(notifiedBlocks[i], string(ep.notifiedBlocks[i][j]))
		}
	}

	prevUpdateStatusBlocks := make(map[string]string)
	for i := range ep.prevUpdateStatusBlocks {
		prevUpdateStatusBlocks[i] = string(ep.prevUpdateStatusBlocks[i])
	}

	err := ep.Storage.UpdateTaskBlocksData(ctx, &db.UpdateTaskBlocksDataRequest{
		Id:                     ep.TaskID,
		ActiveBlocks:           ep.ActiveBlocks,
		SkippedBlocks:          ep.SkippedBlocks,
		NotifiedBlocks:         notifiedBlocks,
		PrevUpdateStatusBlocks: prevUpdateStatusBlocks,
	})
	if err != nil {
		err = errors.Wrap(err, "can`t dump task blocks data")
	}

	return err
}

func (ep *ExecutablePipeline) handleInitiatorNotification(ctx c.Context, step string) error {
	log := logger.GetLogger(ctx)

	if ep.notifiedBlocks == nil {
		ep.notifiedBlocks = make(map[string][]TaskHumanStatus)
	}
	currBlock := ep.Blocks[step]
	currStatus := currBlock.GetTaskHumanStatus()
	ss := ep.notifiedBlocks[step]
	if ss == nil {
		ss = make([]TaskHumanStatus, 0)
	}
	for _, s := range ss {
		if s == currStatus {
			return nil
		}
	}
	descr := ep.currDescription
	additionalDescriptions, err := ep.Storage.GetAdditionalForms(ep.WorkNumber, "")
	if err != nil {
		return err
	}
	for _, item := range additionalDescriptions {
		if item == "" {
			continue
		}
		descr = fmt.Sprintf("%s\n\n%s", descr, item)
	}

	switch currStatus {
	case StatusApproved, StatusApprovementRejected, StatusExecution, StatusExecutionRejected, StatusDone:
		tmpl := mail.NewApplicationInitiatorStatusNotification(
			ep.WorkNumber,
			ep.Name,
			statusToTaskState[currStatus],
			descr,
			ep.Sender.SdAddress)
		if ep.initiatorEmail == "" {
			email, err := ep.People.GetUserEmail(ctx, ep.Initiator)
			if err != nil {
				return err
			}
			ep.initiatorEmail = email
		}

		log.Info("initiatorEmail: ", ep.initiatorEmail)
		if err := ep.Sender.SendNotification(ctx, []string{ep.initiatorEmail}, nil, tmpl); err != nil {
			return err
		}
		ss = append(ss, currStatus)
		ep.notifiedBlocks[step] = ss // TODO: dump somewhere?
	default:
		return nil
	}
	return nil
}

type stepCtx struct {
	workNumber  string
	workTitle   string
	description string
	stepStart   time.Time
}

func (ep *ExecutablePipeline) stepCtx(start time.Time) *stepCtx {
	return &stepCtx{stepStart: start, workNumber: ep.WorkNumber, workTitle: ep.Name, description: ep.currDescription}
}

func (ep *ExecutablePipeline) handleSkippedBlocks(ctx c.Context, runCtx *store.VariableStore) error {
	for step := range ep.SkippedBlocks {
		currentBlock, ok := ep.Blocks[step]
		if !ok || currentBlock == nil {
			continue
		}
		ep.StepType = currentBlock.GetType()

		var err error

		if currentBlock.IsScenario() {
			// TODO good
		} else {
			_, _, err = ep.createStep(ctx, step, false, StatusSkipped)
			if err != nil {
				return err
			}

			nexts, _ := currentBlock.Next(ep.VarStore)
			skipped := currentBlock.Skipped(runCtx)
			delete(ep.SkippedBlocks, step)
			ep.MergeSkippedBlocks(nexts)
			ep.MergeSkippedBlocks(skipped)
		}
	}
	return nil
}

func (ep *ExecutablePipeline) deleteActiveBlock(step string) {
	if ep.ActiveBlocks != nil {
		delete(ep.ActiveBlocks, step)
	}
	if ep.prevUpdateStatusBlocks != nil {
		delete(ep.prevUpdateStatusBlocks, step) // we may want to rerun block later (and change pipeline status)
	}
}

//nolint:gocognit,gocyclo //its really complex
func (ep *ExecutablePipeline) DebugRun(ctx c.Context, _ *stepCtx, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "pipeline_flow")
	defer s.End()

	ep.VarStore = runCtx

	log := logger.GetLogger(ctx)

	if ep.ReadyToStart() {
		ep.ActiveBlocks[ep.EntryPoint] = void{}
	}

	errChange := ep.Storage.ChangeTaskStatus(ctx, ep.TaskID, db.RunStatusRunning)
	if errChange != nil {
		return errChange
	}

	errUpdate := ep.updateStatusByStep(ctx, "", ep.GetTaskHumanStatus())
	if errUpdate != nil {
		return errUpdate
	}
	for !ep.IsOver() {
		for step := range ep.ActiveBlocks {
			if err := ep.handleSkippedBlocks(ctx, runCtx); err != nil {
				return err
			}

			currentBlock, ok := ep.Blocks[step]
			if !ok || currentBlock == nil {
				_, _, err := ep.createStep(ctx, step, true, StatusFinished)
				if err != nil {
					return err
				}

				return errUnknownBlock
			}

			ep.StepType = currentBlock.GetType()

			// initialize step state
			if _, ok = ep.VarStore.State[step]; !ok {
				state, stateErr := json.Marshal(currentBlock.GetState())
				if stateErr != nil {
					return stateErr
				}
				ep.VarStore.ReplaceState(step, state)
			}

			var id uuid.UUID
			var err error
			var ts time.Time

			if currentBlock.IsScenario() {
				// TODO: handle
			} else {
				id, ts, err = ep.createStep(ctx, step, false, StatusIdle)
				if err != nil {
					return err
				}

				sCtx := ep.stepCtx(ts)

				// завершаем запущенный блок, если на другом блоке в этом цикле возникло неуспешное выполнение
				if ep.endExecution {
					updErr := ep.updateStep(ctx, id, err != nil, StatusCancel)
					if updErr != nil {
						return updErr
					}
					ep.deleteActiveBlock(step)
					continue
				}

				errUpdate = ep.updateStatusByStep(ctx, step, currentBlock.GetTaskHumanStatus())
				if errUpdate != nil {
					return errUpdate
				}

				ep.VarStore.SetValue(getWorkIdKey(step), id)

				err = currentBlock.DebugRun(ctx, sCtx, ep.VarStore)
				if err != nil {
					key := step + KeyDelimiter + ErrorKey
					ep.VarStore.SetValue(key, err.Error())
				}
			}

			updErr := ep.updateStep(ctx, id, err != nil, currentBlock.GetStatus())
			if updErr != nil {
				return updErr
			}

			errUpdate = ep.updateStatusByStep(ctx, step, currentBlock.GetTaskHumanStatus())
			if errUpdate != nil {
				return errUpdate
			}

			if errNotif := ep.handleInitiatorNotification(ctx, step); errNotif != nil {
				log.WithError(errNotif).Error("couldn't notify initiator")
			}

			if errDump := ep.dumpTaskBlocksData(ctx); errDump != nil {
				return errDump
			}

			switch currentBlock.GetStatus() {
			case StatusFinished, StatusNoSuccess, StatusCancel:
			default:
				continue
			}

			ep.deleteActiveBlock(step)

			if currentBlock.GetStatus() == StatusCancel {
				ep.endExecution = true
				continue
			}

			switch currentBlock.GetType() {
			case BlockGoEndId:
				ep.endExecution = true
				continue
			case BlockGoSdApplicationID:
				state, exists := ep.VarStore.GetState(step)
				if exists {
					var stateData ApplicationData
					if err = json.Unmarshal(state.(json.RawMessage), &stateData); err != nil {
						log.WithError(err).Error("couldn't get application state")
					} else {
						ep.currDescription = stateData.Description
					}
				}
			}

			activeBlocks, ok := currentBlock.Next(ep.VarStore)
			if !ok {
				updStepErr := ep.updateStep(ctx, id, true, StatusFinished)
				if updStepErr != nil {
					return updStepErr
				}

				return ErrCantGetNextStep
			}
			ep.MergeActiveBlocks(activeBlocks)

			skipped := currentBlock.Skipped(ep.VarStore)
			ep.MergeSkippedBlocks(skipped)

			if errDump := ep.dumpTaskBlocksData(ctx); errDump != nil {
				return errDump
			}

			if runCtx.StopPoints.IsStopPoint(step) {
				errChangeStopped := ep.changeTaskStatus(ctx, db.RunStatusStopped)
				if errChangeStopped != nil {
					return errChange
				}

				return nil
			}
		}
		// prevent spamming
		// TODO: rewrite
		time.Sleep(2 * time.Second)
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

func (ep *ExecutablePipeline) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(ep.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (ep *ExecutablePipeline) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (ep *ExecutablePipeline) GetState() interface{} {
	return nil
}

func (ep *ExecutablePipeline) Update(_ c.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (ep *ExecutablePipeline) CreateBlocks(ctx c.Context, source map[string]entity.EriusFunc) error {
	ep.Blocks = make(map[string]Runner)

	ctx, s := trace.StartSpan(ctx, "create_blocks")
	defer s.End()

	for k := range source {
		bl := source[k]

		block, err := ep.CreateBlock(ctx, k, &bl)
		if err != nil {
			return err
		}

		ep.Blocks[k] = block
	}

	return nil
}

//nolint:gocyclo //ok
func (ep *ExecutablePipeline) CreateBlock(ctx c.Context, name string, bl *entity.EriusFunc) (Runner, error) {
	ctx, s := trace.StartSpan(ctx, "create_block")
	defer s.End()

	switch bl.BlockType {
	case script.TypeGo:
		return ep.CreateGoBlock(ctx, bl, name)
	case script.TypeExternal:
		return createExecutableFunctionBlock(name, bl)
	case script.TypeScenario:
		p, err := ep.Storage.GetExecutableByName(ctx, bl.Title)
		if err != nil {
			return nil, err
		}

		epi := ExecutablePipeline{}
		epi.PipelineID = p.ID
		epi.VersionID = p.VersionID
		epi.Storage = ep.Storage
		epi.EntryPoint = p.Pipeline.Entrypoint
		epi.FaaS = ep.FaaS
		epi.Input = make(map[string]string)
		epi.Output = make(map[string]string)
		epi.Nexts = bl.Next
		epi.Name = bl.Title
		epi.PipelineModel = p

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

//nolint:gocyclo //need bigger cyclomatic
func (ep *ExecutablePipeline) CreateGoBlock(ctx c.Context, ef *entity.EriusFunc, name string) (Runner, error) {
	switch ef.TypeID {
	case BlockGoIfID:
		return createGoIfBlock(name, ef)
	case BlockGoTestID:
		return createGoTestBlock(name, ef), nil
	case BlockGoApproverID:
		return createGoApproverBlock(ctx, name, ef, ep)
	case BlockGoSdApplicationID:
		return createGoSdApplicationBlock(name, ef)
	case BlockGoExecutionID:
		return createGoExecutionBlock(ctx, name, ef, ep)
	case BlockGoStartId:
		return createGoStartBlock(name, ef), nil
	case BlockGoEndId:
		return createGoEndBlock(name, ef), nil
	case BlockWaitForAllInputsId:
		return createGoWaitForAllInputsBlock(name, ef, ep), nil
	case BlockGoBeginParallelTaskId:
		return createGoStartParallelBlock(name, ef), nil
	case BlockGoNotificationID:
		return createGoNotificationBlock(name, ef, ep)
	case BlockExecutableFunctionID:
		return createExecutableFunctionBlock(name, ef)
	case BlockGoFormID:
		return createGoFormBlock(name, ef, ep)
	}

	return nil, errors.New("unknown go-block type: " + ef.TypeID)
}

func getWorkIdKey(stepName string) string {
	return stepName + "." + keyStepWorkId
}
