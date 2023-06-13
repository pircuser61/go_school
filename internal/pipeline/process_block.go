package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
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
	Integrations       *integrations.Service
	FileRegistry       *file_registry.Service
	FaaS               string
	VarStore           *store.VariableStore
	UpdateData         *script.BlockUpdateData
	skipNotifications  bool // for tests
	skipProduce        bool // for tests too :)
	currBlockStartTime time.Time
	Delegations        human_tasks.Delegations
}

func (runCtx *BlockRunContext) Copy() *BlockRunContext {
	runCtxCopy := *runCtx
	//nolint:govet // declare new mutex on next line
	runCtxCopy.VarStore = runCtx.VarStore.Copy()
	runCtxCopy.UpdateData = nil
	return &runCtxCopy
}

//nolint:gocyclo //todo: need to decompose
func processBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) (err error) {
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

	taskHumanStatus := block.GetTaskHumanStatus()
	err = runCtx.updateStatusByStep(ctx, taskHumanStatus)
	if err != nil {
		return err
	}

	isArchived, err := runCtx.Storage.CheckIsArchived(ctx, runCtx.TaskID)
	if err != nil {
		return err
	}

	if isArchived || (block.GetStatus() != StatusFinished && block.GetStatus() != StatusNoSuccess) {
		return nil
	}

	err = runCtx.handleInitiatorNotification(ctx, name, bl.TypeID, taskHumanStatus)
	if err != nil {
		return err
	}

	activeBlocks, ok := block.Next(runCtx.VarStore)
	if !ok {
		err = runCtx.updateStepInDB(ctx, name, id, true, block.GetStatus(), block.Members(), []Deadline{})
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
		err = processBlock(ctx, b, blockData, runCtx.Copy(), false)
		if err != nil {
			return
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
	case BlockPlaceholderID:
		return createGoPlaceholderBlock(name, ef, runCtx), nil
	}

	return nil, errors.New("unknown go-block type: " + ef.TypeID)
}

func initBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, uuid.UUID, error) {
	block, err := CreateBlock(ctx, name, bl, runCtx)
	if err != nil {
		return nil, uuid.Nil, err
	}

	_, blockExists := runCtx.VarStore.State[name]

	// либо блока нет либо блок уже есть и мы зашли в него повторно
	if !blockExists || (blockExists && block.IsReEntered()) {
		state, stateErr := json.Marshal(block.GetState())
		if stateErr != nil {
			return nil, uuid.Nil, stateErr
		}
		runCtx.VarStore.ReplaceState(name, state)
	}

	runCtx.currBlockStartTime = time.Now() // will be used only for the block creation
	deadlines := block.Deadlines()
	id, startTime, err := runCtx.saveStepInDB(ctx, name, bl.TypeID, string(block.GetStatus()),
		block.Members(), deadlines, block.IsReEntered())
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
	deadlines := block.Deadlines()
	err = runCtx.updateStepInDB(ctx, name, id, err != nil, block.GetStatus(), block.Members(), deadlines)
	if err != nil {
		return err
	}

	return nil
}

func (runCtx *BlockRunContext) saveStepInDB(ctx c.Context, name, stepType, status string,
	pl []Member, deadlines []Deadline, isReEntered bool) (uuid.UUID, time.Time, error) {
	storageData, errSerialize := json.Marshal(runCtx.VarStore)
	if errSerialize != nil {
		return db.NullUuid, time.Time{}, errSerialize
	}
	dbPeople := make([]db.DbMember, 0, len(pl))
	dbDeadlines := make([]db.DbDeadline, 0, len(deadlines))
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

	for i := range deadlines {
		dbDeadlines = append(dbDeadlines, db.DbDeadline{
			Action:   string(deadlines[i].Action),
			Deadline: deadlines[i].Deadline,
		})
	}
	return runCtx.Storage.SaveStepContext(ctx, &db.SaveStepRequest{
		WorkID:      runCtx.TaskID,
		StepType:    stepType,
		StepName:    name,
		Content:     storageData,
		BreakPoints: []string{},
		HasError:    false,
		Status:      status,
		Members:     dbPeople,
		Deadlines:   dbDeadlines,
		IsReEntered: isReEntered,
	})
}

func (runCtx *BlockRunContext) updateStepInDB(ctx c.Context, name string, id uuid.UUID, hasError bool, status Status,
	pl []Member, deadlines []Deadline) error {
	storageData, err := json.Marshal(runCtx.VarStore)
	if err != nil {
		return err
	}
	dbPeople := make([]db.DbMember, 0, len(pl))
	dbDeadlines := make([]db.DbDeadline, 0, len(deadlines))
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
	for i := range deadlines {
		dbDeadlines = append(dbDeadlines, db.DbDeadline{
			Action:   string(deadlines[i].Action),
			Deadline: deadlines[i].Deadline,
		})
	}
	return runCtx.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          id,
		StepName:    name,
		Content:     storageData,
		BreakPoints: []string{},
		HasError:    hasError,
		Status:      string(status),
		Members:     dbPeople,
		Deadlines:   dbDeadlines,
	})
}

func (runCtx *BlockRunContext) makeNotificationDescription(nodeName string) (string, error) {
	descr, err := runCtx.Storage.GetApplicationData(runCtx.WorkNumber)
	if err != nil {
		return "", err
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

	var emailAttachment []e.Attachment

	description, err := runCtx.makeNotificationDescription(step)
	if err != nil {
		return err
	}

	loginsToNotify := []string{runCtx.Initiator}

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = runCtx.People.GetUserEmail(ctx, login)
		if err != nil {
			return err
		}

		emails = append(emails, email)
	}

	tmpl := mail.NewAppInitiatorStatusNotificationTpl(
		runCtx.WorkNumber,
		statusToTaskState[status],
		description,
		runCtx.Sender.SdAddress)

	if sendErr := runCtx.Sender.SendNotification(ctx, emails, emailAttachment, tmpl); sendErr != nil {
		return sendErr
	}

	return nil
}

func ProcessBlockWithEndMapping(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_with_end_mapping")
	defer s.End()
	log := logger.GetLogger(ctx)

	pErr := processBlock(ctx, name, bl, runCtx, manual)
	if pErr != nil {
		return pErr
	}
	intStatus, stringStatus, err := runCtx.Storage.GetTaskStatusWithReadableString(ctx, runCtx.TaskID)
	if err != nil {
		log.WithError(err)
		return nil
	}

	if intStatus != 2 && intStatus != 4 {
		return nil
	}

	endErr := processBlockEnd(ctx, stringStatus, runCtx)
	if endErr != nil {
		log.WithError(err)
	}
	return nil
}

func processBlockEnd(ctx c.Context, status string, runCtx *BlockRunContext) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_end")
	defer s.End()

	version, versErr := runCtx.Storage.GetVersionByWorkNumber(ctx, runCtx.WorkNumber)
	if versErr != nil {
		return versErr
	}
	systemsIds, sysIdErr := runCtx.Storage.GetExternalSystemsIDs(ctx, version.VersionID.String())
	if sysIdErr != nil {
		return sysIdErr
	}
	context, contextErr := runCtx.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if contextErr != nil {
		return contextErr
	}
	systemsNames, namesErr := runCtx.Integrations.GetSystemsNames(ctx, systemsIds)
	if namesErr != nil {
		return namesErr
	}
	for key, val := range systemsNames {
		if val == context.ClientID {
			systemSettings, sysErr := runCtx.Storage.GetExternalSystemSettings(ctx, version.VersionID.String(), key)
			if sysErr != nil {
				return sysErr
			}
			if systemSettings.OutputSettings.Method == "" ||
				systemSettings.OutputSettings.URL == "" ||
				systemSettings.OutputSettings.MicroserviceId == "" {
				return nil
			}
			taskTime, timeErr := runCtx.Storage.GetTaskInWorkTime(ctx, runCtx.WorkNumber)
			if timeErr != nil {
				return timeErr
			}
			sendingErr := sendEndingMapping(ctx, val, &entity.EndProcessData{
				Id:         runCtx.TaskID.String(),
				VersionId:  version.VersionID.String(),
				StartedAt:  taskTime.StartedAt.String(),
				FinishedAt: taskTime.FinishedAt.String(),
				Status:     status,
			}, runCtx, systemSettings.OutputSettings)
			if sendingErr != nil {
				return sendingErr
			}
		}
	}
	return nil
}

func sendEndingMapping(ctx c.Context, clientId string, data *entity.EndProcessData,
	runCtx *BlockRunContext, settings *entity.EndSystemSettings) (err error) {
	auth, authErr := runCtx.Integrations.FillAuth(ctx, clientId)
	if authErr != nil {
		return authErr
	}
	body, jsonErr := json.Marshal(data)
	if jsonErr != nil {
		return jsonErr
	}
	req, reqErr := http.NewRequest(settings.Method, settings.URL, bytes.NewBuffer(body))
	if reqErr != nil {
		return reqErr
	}
	if auth.AuthType == "oAuth" {
		bearer := "Bearer " + auth.Token
		req.Header.Add("Authorization", bearer)
		resp, err := runCtx.Integrations.Cli.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	} else {
		req.SetBasicAuth(auth.Login, auth.Password)
		resp, err := runCtx.Integrations.Cli.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}
