package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type BlockRunContext struct {
	TaskID             uuid.UUID
	WorkNumber         string
	WorkTitle          string
	Initiator          string
	Storage            db.Database
	Sender             *mail.Service
	Kafka              *kafka.Service
	People             *people.Service
	ServiceDesc        *servicedesc.Service
	FunctionStore      *functions.Service
	HumanTasks         *human_tasks.Service
	FaaS               string
	VarStore           *store.VariableStore
	UpdateData         *script.BlockUpdateData
	skipNotifications  bool // for tests
	currBlockStartTime time.Time
	Delegations        human_tasks.Delegations
}

func (runCtx *BlockRunContext) Copy() *BlockRunContext {
	runCtxCopy := &(*runCtx)
	runCtxCopy.VarStore = &(*runCtx.VarStore)
	runCtxCopy.UpdateData = nil
	return runCtxCopy
}

//nolint:gocyclo //todo: need to decompose
func ProcessBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("workNumber", runCtx.WorkNumber)

	defer func() {
		if err != nil && !errors.Is(err, UserIsNotPartOfProcessErr{}) {
			log.WithError(err).Error("couldn't process block")
			if changeErr := runCtx.updateTaskStatus(ctx, db.RunStatusError); changeErr != nil {
				log.WithError(changeErr).Error("couldn't change task status")
			}
		}
	}()

	status, getErr := runCtx.Storage.GetTaskStatus(ctx, runCtx.TaskID)
	if getErr != nil {
		err = getErr
		return
	}
	switch status {
	case db.RunStatusCreated:
		if changeErr := runCtx.updateTaskStatus(ctx, db.RunStatusRunning); changeErr != nil {
			err = changeErr
			return
		}
	case db.RunStatusRunning:
	default:
		return nil
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
	err = runCtx.updateStatusByStep(ctx, block.GetTaskHumanStatus())
	if err != nil {
		return err
	}
	if block.GetStatus() == StatusFinished || block.GetStatus() == StatusNoSuccess {
		err = runCtx.handleInitiatorNotification(ctx, name, bl.TypeID, block.GetTaskHumanStatus())
		if err != nil {
			return err
		}
		activeBlocks, ok := block.Next(runCtx.VarStore)
		if !ok {
			err = runCtx.updateStepInDB(ctx, name, id, true, block.GetStatus(), block.Members(),
				false, time.Time{}, false, time.Time{})
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
		return createGoWaitForAllInputsBlock(ctx, name, ef, runCtx)
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

	runCtx.currBlockStartTime = time.Now() // will be used only for the block creation
	checkSLA, checkHalfSLA, deadline, halfDeadline := block.CheckSLA()
	id, startTime, err := runCtx.saveStepInDB(ctx, name, bl.TypeID, string(block.GetStatus()),
		block.Members(), checkSLA, deadline, checkHalfSLA, halfDeadline)
	if err != nil {
		return nil, uuid.Nil, err
	}
	runCtx.currBlockStartTime = startTime
	return block, id, nil
}

func updateBlock(ctx c.Context, block Runner, name string, id uuid.UUID, runCtx *BlockRunContext) error {
	_, err := block.Update(ctx)
	if err != nil {
		return err
	}
	checkSLA, checkHalfSLA, deadline, halfDeadline := block.CheckSLA()
	err = runCtx.updateStepInDB(ctx, name, id, err != nil, block.GetStatus(), block.Members(), checkSLA, deadline, checkHalfSLA, halfDeadline)
	if err != nil {
		return err
	}

	return nil
}

func (runCtx *BlockRunContext) saveStepInDB(ctx c.Context, name, stepType, status string,
	pl []Member, checkSLA bool, deadline time.Time, checkHalfSLA bool, halfDeadline time.Time) (uuid.UUID, time.Time, error) {
	storageData, errSerialize := json.Marshal(runCtx.VarStore)
	if errSerialize != nil {
		return db.NullUuid, time.Time{}, errSerialize
	}
	dbPeople := make([]db.DbMember, 0, len(pl))
	for i := range pl {
		actions := make([]db.DbMemberAction, 0, len(pl[i].Actions))
		for _, act := range pl[i].Actions {
			actions = append(actions, db.DbMemberAction{
				Id:   act.Id,
				Type: act.Type,
			})
		}
		dbPeople = append(dbPeople, db.DbMember{
			Login:    pl[i].Login,
			Finished: pl[i].IsFinished,
			Actions:  actions,
		})
	}
	return runCtx.Storage.SaveStepContext(ctx, &db.SaveStepRequest{
		WorkID:          runCtx.TaskID,
		StepType:        stepType,
		StepName:        name,
		Content:         storageData,
		BreakPoints:     []string{},
		HasError:        false,
		Status:          status,
		Members:         dbPeople,
		CheckSLA:        checkSLA,
		CheckHalfSLA:    checkHalfSLA,
		SLADeadline:     deadline,
		HalfSLADeadline: halfDeadline,
	})
}

func (runCtx *BlockRunContext) updateStepInDB(ctx c.Context, name string, id uuid.UUID, hasError bool, status Status,
	pl []Member, checkSLA bool, deadline time.Time, checkHalfSLA bool, halfDeadline time.Time) error {
	storageData, err := json.Marshal(runCtx.VarStore)
	if err != nil {
		return err
	}
	dbPeople := make([]db.DbMember, 0, len(pl))
	for i := range pl {
		actions := make([]db.DbMemberAction, 0, len(pl[i].Actions))
		for _, act := range pl[i].Actions {
			actions = append(actions, db.DbMemberAction{
				Id:   act.Id,
				Type: act.Type,
			})
		}
		dbPeople = append(dbPeople, db.DbMember{
			Login:    pl[i].Login,
			Finished: pl[i].IsFinished,
			Actions:  actions,
		})
	}
	return runCtx.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:              id,
		StepName:        name,
		Content:         storageData,
		BreakPoints:     []string{},
		HasError:        hasError,
		Status:          string(status),
		Members:         dbPeople,
		CheckSLA:        checkSLA,
		CheckHalfSLA:    checkHalfSLA,
		SLADeadline:     deadline,
		HalfSLADeadline: halfDeadline,
	})
}

func (runCtx *BlockRunContext) makeNotificationDescription(nodeName string) (string, error) {
	data, err := runCtx.Storage.GetApplicationData(runCtx.WorkNumber)
	if err != nil {
		return "", err
	}
	var descr string
	if data != nil {
		dataDescr, ok := data.Get("description")
		if ok {
			convDescr, convOk := dataDescr.(string)
			if convOk {
				descr = convDescr
			}
		}
	}
	additionalDescriptions, err := runCtx.Storage.GetAdditionalForms(runCtx.WorkNumber, nodeName)
	if err != nil {
		return "", err
	}
	for _, item := range additionalDescriptions {
		if item == "" {
			continue
		}
		descr = fmt.Sprintf("%s\n\n%s", descr, item)
	}
	return descr, nil
}

func (runCtx *BlockRunContext) handleInitiatorNotification(ctx c.Context,
	step, stepType string, status TaskHumanStatus) error {
	const FormStepType = "form"

	if runCtx.skipNotifications {
		return nil
	}

	switch status {
	case StatusNew,
		StatusApproved,
		StatusApproveViewed,
		StatusApproveInformed,
		StatusApproveSigned,
		StatusApproveConfirmed,
		StatusApprovementRejected,
		StatusExecution,
		StatusExecutionRejected,
		StatusDone:
	default:
		return nil
	}

	if status == StatusDone && stepType == FormStepType {
		return nil
	}

	descr, err := runCtx.makeNotificationDescription(step)
	if err != nil {
		return err
	}

	delegates, err := runCtx.HumanTasks.GetDelegationsFromLogin(ctx, runCtx.Initiator)
	if err != nil {
		return err
	}

	loginsToNotify := delegates.GetUserInArrayWithDelegations([]string{runCtx.Initiator})

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = runCtx.People.GetUserEmail(ctx, login)
		if err != nil {
			return err
		}

		emails = append(emails, email)
	}

	tmpl := mail.NewApplicationInitiatorStatusNotification(
		runCtx.WorkNumber,
		runCtx.WorkTitle,
		statusToTaskState[status],
		descr,
		runCtx.Sender.SdAddress)

	if sendErr := runCtx.Sender.SendNotification(ctx, emails, nil, tmpl); sendErr != nil {
		return sendErr
	}

	return nil
}
