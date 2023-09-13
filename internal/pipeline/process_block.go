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

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type TaskSubscriptionData struct {
	TaskRunClientID    string
	SystemID           string
	MicroserviceID     string
	MicroserviceURL    string
	NotificationPath   string
	Mapping            script.JSONSchemaProperties
	NotificationSchema script.JSONSchema
	ExpectedEvents     []entity.NodeSubscriptionEvents
}

type RunContextServices struct {
	Storage       db.Database
	Sender        *mail.Service
	Kafka         *kafka.Service
	People        *people.Service
	ServiceDesc   *servicedesc.Service
	FunctionStore *functions.Service
	HumanTasks    *human_tasks.Service
	Integrations  *integrations.Service
	FileRegistry  *file_registry.Service
	FaaS          string
	HrGate        *hrgate.Service
	Scheduler     *scheduler.Service
	SLAService    sla.Service
}

type BlockRunResults struct {
	NodeEvents []entity.NodeEvent
}

type BlockRunContext struct {
	TaskID      uuid.UUID
	WorkNumber  string
	WorkTitle   string
	Initiator   string
	IsTest      bool
	NotifName   string
	Delegations human_tasks.Delegations

	VarStore   *store.VariableStore
	UpdateData *script.BlockUpdateData

	CurrBlockStartTime time.Time

	skipNotifications bool // for tests
	skipProduce       bool // for tests too :)

	Services        RunContextServices
	BlockRunResults *BlockRunResults

	TaskSubscriptionData TaskSubscriptionData
}

func (runCtx BlockRunContext) GetCancelledStepsEvents(ctx c.Context) ([]entity.NodeEvent, error) {
	steps, err := runCtx.Services.Storage.GetCanceledTaskSteps(ctx, runCtx.WorkNumber)
	if err != nil {
		return nil, err
	}

	nodeEvents := make([]entity.NodeEvent, 0, len(steps))

	for _, s := range steps {
		notify := false
		for _, event := range runCtx.TaskSubscriptionData.ExpectedEvents {
			if event.NodeID == s.Name && event.Notify {
				for _, ev := range event.Events {
					if ev == eventEnd {
						notify = true
					}
				}
			}
		}
		if !notify {
			continue
		}
		runCtx.CurrBlockStartTime = s.Time
		event, eventErr := runCtx.MakeNodeEndEvent(ctx, s.Name, StatusRevoke, StatusCanceled)
		if eventErr != nil {
			return nil, eventErr
		}
		nodeEvents = append(nodeEvents, event)
	}

	return nodeEvents, nil
}

func (runCtx *BlockRunContext) FillTaskEvents(ctx c.Context) error {
	taskRunCtx, err := runCtx.Services.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if err != nil {
		return err
	}

	sResp, err := runCtx.Services.Integrations.RpcIntCli.GetIntegrationByClientId(ctx,
		&integration_v1.GetIntegrationByClientIdRequest{ClientId: taskRunCtx.ClientID})
	if err != nil {
		return err
	}
	if sResp == nil || sResp.Integration == nil {
		return nil
	}

	expectedEvents, err := runCtx.Services.Storage.GetTaskEventsParamsByWorkNumber(ctx,
		runCtx.WorkNumber, sResp.Integration.IntegrationId)
	if err != nil {
		return err
	}
	if expectedEvents.SystemID == "" {
		return nil
	}

	mResp, err := runCtx.Services.Integrations.RpcMicrCli.GetMicroservice(ctx,
		&microservice_v1.GetMicroserviceRequest{MicroserviceId: expectedEvents.MicroserviceID})
	if err != nil {
		return err
	}
	if mResp == nil || mResp.Microservice == nil || mResp.Microservice.Creds == nil || mResp.Microservice.Creds.Prod == nil {
		return nil
	}

	runCtx.TaskSubscriptionData.TaskRunClientID = taskRunCtx.ClientID
	runCtx.TaskSubscriptionData.SystemID = sResp.Integration.IntegrationId
	runCtx.TaskSubscriptionData.MicroserviceID = expectedEvents.MicroserviceID
	runCtx.TaskSubscriptionData.MicroserviceURL = mResp.Microservice.Creds.Prod.Addr
	runCtx.TaskSubscriptionData.NotificationPath = expectedEvents.Path
	runCtx.TaskSubscriptionData.Mapping = expectedEvents.Mapping
	runCtx.TaskSubscriptionData.NotificationSchema = expectedEvents.NotificationSchema
	runCtx.TaskSubscriptionData.ExpectedEvents = expectedEvents.Nodes
	return nil
}

func (runCtx *BlockRunContext) Copy() *BlockRunContext {
	runCtxCopy := *runCtx
	//nolint:govet // declare new mutex on next line
	runCtxCopy.VarStore = runCtx.VarStore.Copy()
	runCtxCopy.UpdateData = nil
	return &runCtxCopy
}

//nolint:gocyclo //todo: need to decompose
func processBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext,
	manual bool) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("workNumber", runCtx.WorkNumber)

	defer func() {
		if err != nil && !errors.Is(err, UserIsNotPartOfProcessErr{}) {
			log.WithError(err).Error("couldn't process block")
			if changeErr := runCtx.updateTaskStatus(ctx, db.RunStatusError, "", db.SystemLogin); changeErr != nil {
				log.WithError(changeErr).Error("couldn't change task status")
			}
		}
	}()

	status, getErr := runCtx.Services.Storage.GetTaskStatus(ctx, runCtx.TaskID)
	if getErr != nil {
		err = getErr
		return
	}

	switch status {
	case db.RunStatusCreated:
		if changeErr := runCtx.updateTaskStatus(ctx, db.RunStatusRunning, "", db.SystemLogin); changeErr != nil {
			err = changeErr
			return
		}
	case db.RunStatusRunning:
	case db.RunStatusCanceled:
		return errors.New("couldn't process canceled block")
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

	newEvents := block.GetNewEvents()
	runCtx.BlockRunResults.NodeEvents = append(runCtx.BlockRunResults.NodeEvents, newEvents...)

	isArchived, err := runCtx.Services.Storage.CheckIsArchived(ctx, runCtx.TaskID)
	if err != nil {
		return err
	}

	if isArchived || (block.GetStatus() != StatusFinished && block.GetStatus() != StatusNoSuccess &&
		block.GetStatus() != StatusError) {
		return nil
	}

	err = runCtx.handleInitiatorNotify(ctx, name, bl.TypeID, taskHumanStatus)
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
		blockData, blockErr := runCtx.Services.Storage.GetBlockDataFromVersion(ctx, runCtx.WorkNumber, b)
		if blockErr != nil {
			err = blockErr
			return
		}

		ctxCopy := runCtx.Copy()
		err = processBlock(ctx, b, blockData, ctxCopy, false)
		if err != nil {
			return
		}

		runCtx.BlockRunResults.NodeEvents = append(runCtx.BlockRunResults.NodeEvents, ctxCopy.BlockRunResults.NodeEvents...)
	}

	return nil
}

func CreateBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, bool, error) {
	ctx, s := trace.StartSpan(ctx, "create_block")
	defer s.End()

	expectedEvents := make(map[string]struct{})
	for _, ee := range runCtx.TaskSubscriptionData.ExpectedEvents {
		if ee.NodeID == name && ee.Notify {
			for _, event := range ee.Events {
				expectedEvents[event] = struct{}{}
			}
			break
		}
	}

	switch bl.BlockType {
	case script.TypeGo:
		return createGoBlock(ctx, bl, name, runCtx, expectedEvents)
	case script.TypeExternal:
		return createExecutableFunctionBlock(ctx, name, bl, runCtx, expectedEvents)
	case script.TypeScenario:
		p, err := runCtx.Services.Storage.GetExecutableByName(ctx, bl.Title)
		if err != nil {
			return nil, false, err
		}

		epi := ExecutablePipeline{}
		epi.PipelineID = p.ID
		epi.VersionID = p.VersionID
		epi.Storage = runCtx.Services.Storage
		epi.EntryPoint = p.Pipeline.Entrypoint
		epi.FaaS = runCtx.Services.FaaS
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
			return nil, false, err
		}

		err = epi.CreateTask(ctx, &CreateTaskDTO{
			Author:  "Erius",
			IsDebug: false,
			Params:  parameters,
		})
		if err != nil {
			return nil, false, err
		}

		err = epi.CreateBlocks(ctx, p.Pipeline.Blocks)
		if err != nil {
			return nil, false, err
		}

		for _, v := range bl.Input {
			epi.Input[p.Name+KeyDelimiter+v.Name] = v.Global
		}

		if bl.Output != nil {
			for propertyName, v := range bl.Output.Properties {
				epi.Output[propertyName] = v.Global
			}
		}

		return &epi, false, nil
	}

	return nil, false, errors.Errorf("can't create block with type: %s", bl.BlockType)
}

func createGoBlock(ctx c.Context, ef *entity.EriusFunc, name string, runCtx *BlockRunContext,
	expectedEvents map[string]struct{}) (r Runner, reEntry bool, err error) {
	switch ef.TypeID {
	case BlockGoIfID:
		return createGoIfBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoTestID:
		return createGoTestBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoApproverID:
		return createGoApproverBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoSignID:
		return createGoSignBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoSdApplicationID:
		return createGoSdApplicationBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoExecutionID:
		return createGoExecutionBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoStartId:
		return createGoStartBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoEndId:
		return createGoEndBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockWaitForAllInputsId:
		return createGoWaitForAllInputsBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoBeginParallelTaskId:
		return createGoStartParallelBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoNotificationID:
		return createGoNotificationBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockExecutableFunctionID:
		return createExecutableFunctionBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoFormID:
		return createGoFormBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockPlaceholderID:
		return createGoPlaceholderBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockTimerID:
		return createTimerBlock(ctx, name, ef, runCtx, expectedEvents)
	}

	return nil, false, errors.New("unknown go-block type: " + ef.TypeID)
}

func initBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, uuid.UUID, error) {
	block, isReEntry, err := CreateBlock(ctx, name, bl, runCtx)
	if err != nil {
		return nil, uuid.Nil, err
	}

	_, blockExists := runCtx.VarStore.State[name]

	// либо блока нет либо блок уже есть и мы зашли в него повторно
	if !blockExists || isReEntry {
		state, stateErr := json.Marshal(block.GetState())
		if stateErr != nil {
			return nil, uuid.Nil, stateErr
		}
		runCtx.VarStore.ReplaceState(name, state)
	}

	runCtx.CurrBlockStartTime = time.Now() // will be used only for the block creation
	deadlines, deadlinesErr := block.Deadlines(ctx)
	if deadlinesErr != nil {
		return nil, uuid.Nil, deadlinesErr
	}
	id, startTime, err := runCtx.saveStepInDB(ctx, name, bl.TypeID, string(block.GetStatus()),
		block.Members(), deadlines, isReEntry)
	if err != nil {
		return nil, uuid.Nil, err
	}
	runCtx.CurrBlockStartTime = startTime
	return block, id, nil
}

func updateBlock(ctx c.Context, block Runner, name string, id uuid.UUID, runCtx *BlockRunContext) error {
	_, err := block.Update(ctx)
	if err != nil {
		return err
	}
	deadlines, deadlinesErr := block.Deadlines(ctx)
	if deadlinesErr != nil {
		return deadlinesErr
	}
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
				Id:     act.Id,
				Type:   act.Type,
				Params: act.Params,
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
	return runCtx.Services.Storage.SaveStepContext(ctx, &db.SaveStepRequest{
		WorkID:      runCtx.TaskID,
		StepType:    stepType,
		StepName:    name,
		Content:     storageData,
		BreakPoints: []string{},
		HasError:    false,
		Status:      status,
		Members:     dbPeople,
		Deadlines:   dbDeadlines,
		IsReEntry:   isReEntered,
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
				Id:     act.Id,
				Type:   act.Type,
				Params: act.Params,
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
	return runCtx.Services.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
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
	descr, err := runCtx.Services.Storage.GetApplicationData(runCtx.WorkNumber)
	if err != nil {
		return "", err
	}
	additionalDescriptions, err := runCtx.Services.Storage.GetAdditionalForms(runCtx.WorkNumber, nodeName)
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

func (runCtx *BlockRunContext) handleInitiatorNotify(ctx c.Context,
	step, stepType string, status TaskHumanStatus) error {
	const (
		FormStepType     = "form"
		FunctionStepType = "executable_function"
	)

	if runCtx.skipNotifications {
		return nil
	}

	switch status {
	case StatusNew,
		StatusApproved,
		StatusApproveViewed,
		StatusApproveInformed,
		StatusApproveConfirmed,
		StatusApprovementRejected,
		StatusExecution,
		StatusExecutionRejected,
		StatusSigned,
		StatusRejected,
		StatusProcessingError,
		StatusDone:
	default:
		return nil
	}

	if status == StatusDone && (stepType == FormStepType || stepType == FunctionStepType) {
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
		email, err = runCtx.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			return err
		}

		emails = append(emails, email)
	}
	tmpl := mail.NewAppInitiatorStatusNotificationTpl(
		runCtx.WorkNumber,
		runCtx.NotifName,
		statusToTaskState[status],
		description,
		runCtx.Services.Sender.SdAddress)

	if sendErr := runCtx.Services.Sender.SendNotification(ctx, emails, emailAttachment, tmpl); sendErr != nil {
		return sendErr
	}

	return nil
}

func ProcessBlockWithEndMapping(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext,
	manual bool) error {
	ctx, s := trace.StartSpan(ctx, "process_block_with_end_mapping")
	defer s.End()

	log := logger.GetLogger(ctx)

	runCtx.BlockRunResults = &BlockRunResults{}

	pErr := processBlock(ctx, name, bl, runCtx, manual)
	if pErr != nil {
		return pErr
	}
	intStatus, stringStatus, err := runCtx.Services.Storage.GetTaskStatusWithReadableString(ctx, runCtx.TaskID)
	if err != nil {
		log.WithError(err)
		return nil
	}

	if intStatus != 2 && intStatus != 4 {
		return nil
	}

	endErr := processBlockEnd(ctx, stringStatus, runCtx)
	if endErr != nil {
		log.WithError(endErr)
	}
	return nil
}

func processBlockEnd(ctx c.Context, status string, runCtx *BlockRunContext) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_end")
	defer s.End()

	version, versErr := runCtx.Services.Storage.GetVersionByWorkNumber(ctx, runCtx.WorkNumber)
	if versErr != nil {
		return versErr
	}
	systemsIds, sysIdErr := runCtx.Services.Storage.GetExternalSystemsIDs(ctx, version.VersionID.String())
	if sysIdErr != nil {
		return sysIdErr
	}
	context, contextErr := runCtx.Services.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if contextErr != nil {
		return contextErr
	}
	systemsNames, namesErr := runCtx.Services.Integrations.GetSystemsNames(ctx, systemsIds)
	if namesErr != nil {
		return namesErr
	}
	for key, val := range systemsNames {
		if val == context.ClientID {
			systemSettings, sysErr := runCtx.Services.Storage.GetExternalSystemSettings(ctx, version.VersionID.String(), key)
			if sysErr != nil {
				return sysErr
			}
			if systemSettings.OutputSettings.Method == "" ||
				systemSettings.OutputSettings.URL == "" ||
				systemSettings.OutputSettings.MicroserviceId == "" {
				return nil
			}
			taskTime, timeErr := runCtx.Services.Storage.GetTaskInWorkTime(ctx, runCtx.WorkNumber)
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
	auth, authErr := runCtx.Services.Integrations.FillAuth(ctx, clientId)
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
		resp, err := runCtx.Services.Integrations.Cli.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	} else {
		req.SetBasicAuth(auth.Login, auth.Password)
		resp, err := runCtx.Services.Integrations.Cli.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}
