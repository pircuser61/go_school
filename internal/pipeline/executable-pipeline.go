package pipeline

import (
	"context"
	"encoding/json"
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
	Input         map[string]string
	Output        map[string]string
	Name          string
	PipelineModel *entity.EriusScenario
	HTTPClient    *http.Client
	Remedy        string
	Sender        *mail.Service
	People        *people.Service

	FaaS string

	endExecution bool

	Initiator       string
	initiatorEmail  string
	currDescription string
	notifiedBlocks  map[string][]TaskHumanStatus
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

func (ep *ExecutablePipeline) CreateTask(ctx context.Context, author string, isDebugMode bool, parameters []byte) error {
	ep.TaskID = uuid.New()

	task, err := ep.Storage.CreateTask(ctx, ep.TaskID, ep.VersionID, author, isDebugMode, parameters)
	if err != nil {
		return err
	}

	ep.WorkNumber = task.WorkNumber
	return nil
}

func (ep *ExecutablePipeline) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return ep.DebugRun(ctx, nil, runCtx)
}

func (ep *ExecutablePipeline) createStep(ctx context.Context, name string, hasError bool, status Status) (uuid.UUID, time.Time, error) {
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

func (ep *ExecutablePipeline) updateStep(ctx context.Context, id uuid.UUID, hasError bool, status Status) error {
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

func (ep *ExecutablePipeline) changeTaskStatus(ctx context.Context, taskStatus int) error {
	errChange := ep.Storage.ChangeTaskStatus(ctx, ep.TaskID, taskStatus)
	if errChange != nil {
		ep.VarStore.AddError(errChange)

		return errChange
	}

	return nil
}

// TODO: что-то сделать
func (ep *ExecutablePipeline) updateStatusByStep(c context.Context, status TaskHumanStatus) error {
	if status != "" {
		return ep.Storage.UpdateTaskHumanStatus(c, ep.TaskID, string(status))
	}
	return nil
}

func (ep *ExecutablePipeline) handleInitiatorNotification(c context.Context, step string) error {
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

	switch currStatus {
	case StatusApproved, StatusApprovementRejected, StatusExecution, StatusExecutionRejected, StatusDone:
		tmpl := mail.NewApplicationInitiatorStatusNotification(
			ep.WorkNumber,
			ep.Name,
			statusToTaskState[currStatus],
			ep.currDescription,
			ep.Sender.SdAddress)
		if ep.initiatorEmail == "" {
			email, err := ep.People.GetUserEmail(c, ep.Initiator)
			if err != nil {
				return err
			}
			ep.initiatorEmail = email
		}
		if err := ep.Sender.SendNotification(c, []string{ep.initiatorEmail}, tmpl); err != nil {
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

func (ep *ExecutablePipeline) handleSkippedBlocks(ctx context.Context, runCtx *store.VariableStore) error {
	for step := range ep.SkippedBlocks {
		currentBlock, ok := ep.Blocks[step]
		if !ok || currentBlock == nil {
			continue
		}
		ep.StepType = currentBlock.GetType()

		var err error

		if currentBlock.IsScenario() {
			// TODO
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

//nolint:gocognit,gocyclo //its really complex
func (ep *ExecutablePipeline) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
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

	errUpdate := ep.updateStatusByStep(ctx, ep.GetTaskHumanStatus())
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
					delete(ep.ActiveBlocks, step)
					continue
				}

				errUpdate = ep.updateStatusByStep(ctx, currentBlock.GetTaskHumanStatus())
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

			errUpdate = ep.updateStatusByStep(ctx, currentBlock.GetTaskHumanStatus())
			if errUpdate != nil {
				return errUpdate
			}

			if errNotif := ep.handleInitiatorNotification(ctx, step); errNotif != nil {
				log.WithError(errNotif).Error("couldn't notify initiator")
			}

			switch currentBlock.GetStatus() {
			case StatusFinished, StatusNoSuccess:
			default:
				continue
			}

			delete(ep.ActiveBlocks, step)

			switch currentBlock.GetType() {
			case BlockGoEndId:
				ep.endExecution = true
				continue
			case BlockGoSdApplicationID:
				state, exists := ep.VarStore.GetState(step)
				if exists {
					var stateData ApplicationData
					if err := json.Unmarshal(state.(json.RawMessage), &stateData); err != nil {
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
	nexts, ok := ep.Nexts[DefaultSocket]
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

func (ep *ExecutablePipeline) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (ep *ExecutablePipeline) CreateBlocks(ctx context.Context, source map[string]entity.EriusFunc) error {
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
func (ep *ExecutablePipeline) CreateBlock(ctx context.Context, name string, block *entity.EriusFunc) (Runner, error) {
	ctx, s := trace.StartSpan(ctx, "create_block")
	defer s.End()

	switch block.BlockType {
	case script.TypeGo, BlockGoSdApplicationID, BlockGoApproverID:
		return ep.CreateGoBlock(block, name)
	case script.TypePython3, script.TypePythonFlask, script.TypePythonHTTP:
		fb := FunctionBlock{
			Name:           name,
			Type:           block.BlockType,
			FunctionName:   block.Title,
			FunctionInput:  make(map[string]string),
			FunctionOutput: make(map[string]string),
			Nexts:          block.Next,
			RunURL:         ep.FaaS + "function/%s",
		}

		for _, v := range block.Input {
			fb.FunctionInput[v.Name] = v.Global
		}

		for _, v := range block.Output {
			fb.FunctionOutput[v.Name] = v.Global
		}

		return &fb, nil
	case script.TypeScenario:
		p, err := ep.Storage.GetExecutableByName(ctx, block.Title)
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
		epi.Nexts = block.Next
		epi.Name = block.Title
		epi.PipelineModel = p

		parametersMap := make(map[string]interface{})
		for _, v := range block.Input {
			parametersMap[v.Name] = v.Global
		}

		parameters, err := json.Marshal(parametersMap)
		if err != nil {
			return nil, err
		}

		err = epi.CreateTask(ctx, "Erius", false, parameters)
		if err != nil {
			return nil, err
		}

		err = epi.CreateBlocks(ctx, p.Pipeline.Blocks)
		if err != nil {
			return nil, err
		}

		for _, v := range block.Input {
			epi.Input[p.Name+KeyDelimiter+v.Name] = v.Global
		}

		for _, v := range block.Output {
			epi.Output[v.Name] = v.Global
		}

		return &epi, nil
	}

	return nil, errors.Errorf("can't create block with type: %s", block.BlockType)
}

//nolint:gocyclo //need bigger cyclomatic
func (ep *ExecutablePipeline) CreateGoBlock(ef *entity.EriusFunc, name string) (Runner, error) {
	switch ef.TypeID {
	case BlockGoIfID:
		return createGoIfBlock(name, ef)
	case BlockGoTestID:
		return createGoTestBlock(name, ef), nil
	case BlockGoApproverID:
		return createGoApproverBlock(name, ef, ep)
	case BlockGoSdApplicationID:
		return createGoSdApplicationBlock(name, ef)
	case BlockGoExecutionID:
		return createGoExecutionBlock(name, ef, ep)
	case BlockGoStartId:
		return createGoStartBlock(name, ef), nil
	case BlockGoEndId:
		return createGoEndBlock(name, ef), nil
	case BlockWaitForAllInputsId:
		return createGoWaitForAllInputsBlock(name, ef, ep), nil
	case BlockGoNotificationID:
		return createGoNotificationBlock(name, ef, ep)
	}

	return nil, errors.New("unknown go-block type")
}

func getWorkIdKey(stepName string) string {
	return stepName + "." + keyStepWorkId
}
