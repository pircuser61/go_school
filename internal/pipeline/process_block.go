package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
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
	TaskRunClientID      string
	SystemID             string
	MicroserviceID       string
	MicroserviceURL      string
	MicroserviceAuthType string
	MicroserviceSecrets  map[string]interface{}
	NotificationPath     string
	Method               string
	Mapping              script.JSONSchemaProperties
	NotificationSchema   script.JSONSchema
	ExpectedEvents       []entity.NodeSubscriptionEvents
}

type RunContextServices struct {
	HTTPClient    *retryablehttp.Client
	Storage       db.Database
	Sender        *mail.Service
	Kafka         *kafka.Service
	People        people.Service
	ServiceDesc   servicedesc.Service
	FunctionStore functions.Service
	HumanTasks    human_tasks.Service
	Integrations  integrations.Service
	FileRegistry  fileregistry.Service
	FaaS          string
	JocastaURL    string
	HrGate        hrgate.Service
	Scheduler     *scheduler.Service
	SLAService    sla.Service
}

type BlockRunResults struct {
	NodeEvents      []entity.NodeEvent
	NodeKafkaEvents []entity.NodeKafkaEvent
}

type BlockRunContext struct {
	TaskID                uuid.UUID
	WorkNumber            string
	ClientID              string
	PipelineID            uuid.UUID
	VersionID             uuid.UUID
	WorkTitle             string
	Initiator             string
	IsTest                bool
	CustomTitle           string
	NotifName             string
	Delegations           human_tasks.Delegations
	NotifyProcessFinished bool

	VarStore   *store.VariableStore
	UpdateData *script.BlockUpdateData

	CurrBlockStartTime time.Time

	skipNotifications bool // for tests
	skipProduce       bool // for tests too :)

	Services        RunContextServices
	BlockRunResults *BlockRunResults

	TaskSubscriptionData TaskSubscriptionData

	OnceProductive bool
	Productive     bool
	BreachedSLA    bool
}

func (runCtx *BlockRunContext) Copy() *BlockRunContext {
	runCtxCopy := *runCtx
	//nolint:govet // declare new mutex on next line
	runCtxCopy.VarStore = runCtx.VarStore.Copy()
	runCtxCopy.UpdateData = nil
	runCtxCopy.BlockRunResults = &BlockRunResults{
		NodeEvents:      make([]entity.NodeEvent, 0),
		NodeKafkaEvents: make([]entity.NodeKafkaEvent, 0),
	}
	runCtxCopy.Productive = !runCtx.OnceProductive

	return &runCtxCopy
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
		return createScenarioBlock(ctx, runCtx, bl)
	}

	return nil, false, errors.Errorf("can't create block with type: %s", bl.BlockType)
}

func createGoBlock(ctx c.Context, ef *entity.EriusFunc, name string, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (r Runner, reEntry bool, err error) {
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
	case BlockGoStartID:
		return createGoStartBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoEndID:
		return createGoEndBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockWaitForAllInputsID:
		return createGoWaitForAllInputsBlock(ctx, name, ef, runCtx, expectedEvents)
	case BlockGoBeginParallelTaskID:
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

type Block struct {
	DB            db.Database
	Name          string
	StepType      string
	WorkID        uuid.UUID
	VarStore      *store.VariableStore
	IsPaused      bool
	HasUpdateData bool
	Time          time.Time
}

func (b *Block) FillFromRunContext(runCtx *BlockRunContext) {
	b.DB = runCtx.Services.Storage
	b.WorkID = runCtx.TaskID
	b.VarStore = runCtx.VarStore
	b.IsPaused = runCtx.OnceProductive
	b.HasUpdateData = runCtx.UpdateData != nil
}

func (b *Block) CreateInDB(ctx c.Context) error {
	storageData, err := json.Marshal(b.VarStore)
	if err != nil {
		return err
	}

	if b.Time.IsZero() {
		b.Time = time.Now()
	}

	return b.DB.CreateTaskBlock(ctx, &db.SaveStepRequest{
		WorkID:     b.WorkID,
		StepName:   b.Name,
		StepType:   b.StepType,
		Status:     string(StatusReady),
		Content:    storageData,
		IsPaused:   b.IsPaused,
		HasUpdData: b.HasUpdateData,
		BlockStart: b.Time,
	})
}

func initBlock(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext) (Runner, uuid.UUID, error) {
	exists, id, startTime, err := runCtx.Services.Storage.IsStepExist(ctx, runCtx.TaskID.String(), name, runCtx.UpdateData != nil)
	if err != nil {
		return nil, uuid.Nil, err
	}

	if !exists {
		logger.GetLogger(ctx).
			WithFields(logger.Fields{
				"funcName": "initBlock",
				"workID":   runCtx.TaskID.String(),
				"stepName": name,
				"stepID":   "",
			}).
			Warning("block is not exists")
	}

	if !runCtx.Productive {
		return nil, id, nil
	}

	ctx = logger.WithLogger(ctx, logger.GetLogger(ctx).WithField("stepID", id))

	block, isReEntry, err := CreateBlock(ctx, name, bl, runCtx)
	if err != nil {
		return nil, uuid.Nil, err
	}

	_, blockExistsInContext := runCtx.VarStore.State[name]

	// либо блока нет либо блок уже есть и мы зашли в него повторно
	if !blockExistsInContext || isReEntry {
		state, stateErr := json.Marshal(block.GetState())
		if stateErr != nil {
			return nil, uuid.Nil, stateErr
		}

		runCtx.VarStore.ReplaceState(name, state)
	}

	runCtx.CurrBlockStartTime = startTime

	deadlines, deadlinesErr := block.Deadlines(ctx)
	if deadlinesErr != nil {
		return nil, uuid.Nil, deadlinesErr
	}

	id, err = runCtx.saveStepInDB(ctx, &saveStepDTO{
		name:            name,
		stepType:        bl.TypeID,
		status:          string(block.GetStatus()),
		members:         block.Members(),
		deadlines:       deadlines,
		isReEntered:     isReEntry,
		blockExist:      blockExistsInContext,
		attachments:     block.BlockAttachments(),
		currentExecutor: block.CurrentExecutorData(),
	}, id)
	if err != nil {
		return nil, uuid.Nil, err
	}

	return block, id, nil
}

/*
поскольку значения функции просто пробрасываются дальше
удобно чтобы выходные параметры соответствовали выходным параметрам
функции из которой вызывается эта функция
*/
//nolint:unparam // см выше
func createScenarioBlock(ctx c.Context, runCtx *BlockRunContext, bl *entity.EriusFunc) (*ExecutablePipeline, bool, error) {
	p, err := runCtx.Services.Storage.GetExecutableByName(ctx, bl.Title)
	if err != nil {
		return nil, false, err
	}

	epi := ExecutablePipeline{
		PipelineID:    p.PipelineID,
		VersionID:     p.VersionID,
		Storage:       runCtx.Services.Storage,
		EntryPoint:    p.Pipeline.Entrypoint,
		FaaS:          runCtx.Services.FaaS,
		Input:         make(map[string]string),
		Output:        make(map[string]string),
		Nexts:         bl.Next,
		Name:          bl.Title,
		PipelineModel: p,
		RunContext:    runCtx,
	}

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
		//nolint:gocritic //коллекция без поинтеров
		for propertyName, v := range bl.Output.Properties {
			epi.Output[propertyName] = v.Global
		}
	}

	err = epi.Storage.SetLastRunID(ctx, runCtx.TaskID, epi.VersionID)
	if err != nil {
		return nil, false, errors.Wrap(err, "can’t set id of the last runned task")
	}

	return &epi, false, nil
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

	err = runCtx.updateStepInDB(ctx, &updateStepDTO{
		id:              id,
		name:            name,
		status:          block.GetStatus(),
		hasError:        err != nil,
		members:         block.Members(),
		deadlines:       deadlines,
		attachments:     block.BlockAttachments(),
		currentExecutor: block.CurrentExecutorData(),
	})
	if err != nil {
		return err
	}

	return nil
}

type saveStepDTO struct {
	name, stepType, status string
	members                []Member
	deadlines              []Deadline
	attachments            []string
	isReEntered            bool
	currentExecutor        CurrentExecutorData
	blockExist             bool
}

func (runCtx *BlockRunContext) saveStepInDB(ctx c.Context, dto *saveStepDTO, id uuid.UUID) (uuid.UUID, error) {
	storageData, errSerialize := json.Marshal(runCtx.VarStore)
	if errSerialize != nil {
		return uuid.Nil, errSerialize
	}

	dbMembers := make([]db.Member, 0, len(dto.members))
	dbDeadlines := make([]db.Deadline, 0, len(dto.deadlines))

	for i := range dto.members {
		actions := make([]db.MemberAction, 0, len(dto.members[i].Actions))
		for _, act := range dto.members[i].Actions {
			actions = append(actions, db.MemberAction{
				ID:     act.ID,
				Type:   act.Type,
				Params: act.Params,
			})
		}

		dbMembers = append(dbMembers, db.Member{
			Login:                dto.members[i].Login,
			Actions:              actions,
			IsActed:              dto.members[i].IsActed,
			Finished:             dto.members[i].Finished,
			ExecutionGroupMember: dto.members[i].ExecutionGroupMember,
		})
	}

	for i := range dto.deadlines {
		dbDeadlines = append(dbDeadlines, db.Deadline{
			Action:   string(dto.deadlines[i].Action),
			Deadline: dto.deadlines[i].Deadline,
		})
	}

	return runCtx.Services.Storage.SaveStepContext(ctx, &db.SaveStepRequest{
		WorkID:      runCtx.TaskID,
		StepType:    dto.stepType,
		StepName:    dto.name,
		Content:     storageData,
		BreakPoints: []string{},
		HasError:    false,
		Status:      dto.status,
		Members:     dbMembers,
		Deadlines:   dbDeadlines,
		IsReEntry:   dto.isReEntered,
		BlockExist:  dto.blockExist,
		Attachments: len(dto.attachments),
		CurrentExecutor: db.CurrentExecutorData{
			GroupID:       dto.currentExecutor.GroupID,
			GroupName:     dto.currentExecutor.GroupName,
			People:        dto.currentExecutor.People,
			InitialPeople: dto.currentExecutor.InitialPeople,
			GroupLimit:    dto.currentExecutor.GroupLimit,
		},
		BlockStart: runCtx.CurrBlockStartTime,
	}, id)
}

type updateStepDTO struct {
	id              uuid.UUID
	name            string
	status          Status
	hasError        bool
	members         []Member
	deadlines       []Deadline
	attachments     []string
	currentExecutor CurrentExecutorData
}

func (runCtx *BlockRunContext) updateStepInDB(ctx c.Context, dto *updateStepDTO) error {
	storageData, err := json.Marshal(runCtx.VarStore)
	if err != nil {
		return err
	}

	dbMembers := make([]db.Member, 0, len(dto.members))
	dbDeadlines := make([]db.Deadline, 0, len(dto.deadlines))

	for i := range dto.members {
		actions := make([]db.MemberAction, 0, len(dto.members[i].Actions))
		for _, act := range dto.members[i].Actions {
			actions = append(actions, db.MemberAction{
				ID:     act.ID,
				Type:   act.Type,
				Params: act.Params,
			})
		}

		dbMembers = append(dbMembers, db.Member{
			Login:                dto.members[i].Login,
			Actions:              actions,
			IsActed:              dto.members[i].IsActed,
			Finished:             dto.members[i].Finished,
			ExecutionGroupMember: dto.members[i].ExecutionGroupMember,
			IsInitiator:          dto.members[i].IsInitiator,
		})
	}

	for i := range dto.deadlines {
		dbDeadlines = append(dbDeadlines, db.Deadline{
			Action:   string(dto.deadlines[i].Action),
			Deadline: dto.deadlines[i].Deadline,
		})
	}

	return runCtx.Services.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		ID:          dto.id,
		StepName:    dto.name,
		Content:     storageData,
		BreakPoints: []string{},
		HasError:    dto.hasError,
		Status:      string(dto.status),
		Members:     dbMembers,
		Deadlines:   dbDeadlines,
		Attachments: len(dto.attachments),
		CurrentExecutor: db.CurrentExecutorData{
			GroupID:       dto.currentExecutor.GroupID,
			GroupName:     dto.currentExecutor.GroupName,
			People:        dto.currentExecutor.People,
			InitialPeople: dto.currentExecutor.InitialPeople,
			GroupLimit:    dto.currentExecutor.GroupLimit,
		},
	})
}

func ProcessBlockWithEndMapping(
	ctx c.Context,
	name string,
	bl *entity.EriusFunc,
	runCtx *BlockRunContext,
	manual bool,
) (blockName string, finished bool, err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_with_end_mapping")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("funcName", "ProcessBlockWithEndMapping")

	statusBefore, _, err := runCtx.Services.Storage.GetTaskStatusWithReadableString(ctx, runCtx.TaskID)
	if err != nil {
		log.WithError(err).Error("couldn't get task status before processing")

		return "", false, nil
	}

	runCtx.BlockRunResults = &BlockRunResults{}

	processor := newBlockProcessor(name, bl, runCtx, manual)

	log = log.WithField(script.WorkNumber, processor.runCtx.WorkNumber).
		WithField(script.PipelineID, processor.runCtx.PipelineID).
		WithField(script.VersionID, processor.runCtx.VersionID).
		WithField(script.StepID, processor.runCtx.TaskID).
		WithField(script.StepName, name)

	ctx = logger.WithLogger(ctx, log)

	failedBlock, pErr := processor.ProcessBlock(ctx, 0)
	if pErr != nil {
		log.WithError(pErr).Error("couldn't process block with end mapping, ProcessBlock")

		return failedBlock, false, pErr
	}

	go func() {
		updDeadlineErr := processor.updateTaskExecDeadline(ctx)
		if updDeadlineErr != nil {
			log.WithError(updDeadlineErr).Error("couldn't update task deadline")
		}

		intStatus, stringStatus, err := runCtx.Services.Storage.GetTaskStatusWithReadableString(ctx, runCtx.TaskID)
		if err != nil {
			log.WithError(err).Error("couldn't get task status after processing")
		}

		if intStatus != db.RunStatusFinished && intStatus != db.RunStatusStopped {
			return
		}

		if intStatus == db.RunStatusFinished && statusBefore != db.RunStatusFinished {
			params := struct {
				Steps []string `json:"steps"`
			}{Steps: []string{}}

			jsonParams, mrshErr := json.Marshal(params)
			if mrshErr != nil {
				log.Error(mrshErr)
			}

			_, err = runCtx.Services.Storage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
				WorkID:    runCtx.TaskID.String(),
				EventType: "pause",
				Author:    db.SystemLogin,
				Params:    jsonParams,
			})
			if err != nil {
				log.WithError(updDeadlineErr).Error("couldn't create task event")
			}
		}

		endErr := processBlockEnd(ctx, stringStatus, runCtx)
		if endErr != nil {
			log.WithError(endErr).Error("couldn't send process end notification")
		}
	}()

	return "", true, nil
}

func processBlockEnd(ctx c.Context, status string, runCtx *BlockRunContext) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_end")
	defer s.End()

	log := logger.GetLogger(ctx).WithField(script.FuncName, "processBlockEnd")

	version, versErr := runCtx.Services.Storage.GetVersionByWorkNumber(ctx, runCtx.WorkNumber)
	if versErr != nil {
		return versErr
	}

	systemsIds, sysIDErr := runCtx.Services.Storage.GetExternalSystemsIDs(ctx, version.VersionID.String())
	if sysIDErr != nil {
		return sysIDErr
	}

	context, contextErr := runCtx.Services.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if contextErr != nil {
		return contextErr
	}

	systemsClients, namesErr := runCtx.Services.Integrations.GetSystemsClients(ctx, systemsIds)
	if namesErr != nil {
		return namesErr
	}

	couldSend := false

	for key, cc := range systemsClients {
		clientFound := false

		for _, cli := range cc {
			if cli == context.ClientID {
				clientFound = true

				break
			}
		}

		if !clientFound {
			continue
		}

		systemSettings, sysErr := runCtx.Services.Storage.GetExternalSystemSettings(ctx, version.VersionID.String(), key)
		if sysErr != nil {
			return sysErr
		}

		if systemSettings.OutputSettings.Method == "" ||
			systemSettings.OutputSettings.URL == "" ||
			systemSettings.OutputSettings.MicroserviceID == "" {
			log.Info(fmt.Sprintf("no output settings for clientID %s", context.ClientID))

			return nil
		}

		taskTime, timeErr := runCtx.Services.Storage.GetTaskInWorkTime(ctx, runCtx.WorkNumber)
		if timeErr != nil {
			return timeErr
		}

		sendingErr := sendEndingMapping(ctx, &entity.EndProcessData{
			ID:         runCtx.TaskID.String(),
			VersionID:  version.VersionID.String(),
			StartedAt:  taskTime.StartedAt.String(),
			FinishedAt: taskTime.FinishedAt.String(),
			Status:     status,
		}, runCtx, systemSettings.OutputSettings)
		if sendingErr != nil {
			return sendingErr
		}

		couldSend = true
	}

	if !couldSend {
		log.Info(fmt.Sprintf("found no system for clientID %s to send end process notification", context.ClientID))
	}

	return nil
}

func sendEndingMapping(
	ctx c.Context,
	data *entity.EndProcessData,
	runCtx *BlockRunContext,
	settings *entity.EndSystemSettings,
) (err error) {
	secretsHumanKey, secretsErr := runCtx.Services.Integrations.GetMicroserviceHumanKey(
		ctx,
		settings.MicroserviceID,
		runCtx.PipelineID.String(),
		runCtx.VersionID.String(),
		runCtx.WorkNumber,
		runCtx.ClientID)
	if secretsErr != nil {
		return secretsErr
	}

	auth, authErr := runCtx.Services.Integrations.FillAuth(
		ctx,
		secretsHumanKey,
		runCtx.PipelineID.String(),
		runCtx.VersionID.String(),
		runCtx.WorkNumber,
		runCtx.ClientID)
	if authErr != nil {
		return authErr
	}

	body, jsonErr := json.Marshal(data)
	if jsonErr != nil {
		return jsonErr
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, settings.Method, settings.URL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	integrationsCli := runCtx.Services.Integrations.GetCli()

	if auth.AuthType == "oAuth" {
		bearer := "Bearer " + auth.Token

		req.Header.Add("Authorization", bearer)

		resp, err := integrationsCli.Do(req)
		if err != nil {
			return err
		}

		resp.Body.Close()
	} else {
		req.SetBasicAuth(auth.Login, auth.Password)

		resp, err := integrationsCli.Do(req)
		if err != nil {
			return err
		}

		resp.Body.Close()
	}

	return nil
}
