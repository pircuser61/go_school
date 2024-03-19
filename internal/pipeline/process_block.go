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

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
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
	HTTPClient    *http.Client
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
	NodeEvents      []entity.NodeEvent
	NodeKafkaEvents []entity.NodeKafkaEvent
}

type BlockRunContext struct {
	TaskID      uuid.UUID
	WorkNumber  string
	ClientID    string
	PipelineID  uuid.UUID
	VersionID   uuid.UUID
	WorkTitle   string
	Initiator   string
	IsTest      bool
	CustomTitle string
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

func (runCtx *BlockRunContext) Copy() *BlockRunContext {
	runCtxCopy := *runCtx
	//nolint:govet // declare new mutex on next line
	runCtxCopy.VarStore = runCtx.VarStore.Copy()
	runCtxCopy.UpdateData = nil
	runCtxCopy.BlockRunResults = &BlockRunResults{
		NodeEvents: make([]entity.NodeEvent, 0),
		NodeKafkaEvents: make([]entity.NodeKafkaEvent, 0),
	}

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
		p, err := runCtx.Services.Storage.GetExecutableByName(ctx, bl.Title)
		if err != nil {
			return nil, false, err
		}

		epi := ExecutablePipeline{}
		epi.PipelineID = p.PipelineID
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

	id, startTime, err := runCtx.saveStepInDB(ctx, &saveStepDTO{
		name:            name,
		stepType:        bl.TypeID,
		status:          string(block.GetStatus()),
		members:         block.Members(),
		deadlines:       deadlines,
		isReEntered:     isReEntry,
		attachments:     block.BlockAttachments(),
		currentExecutor: block.CurrentExecutorData(),
	})
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
}

func (runCtx *BlockRunContext) saveStepInDB(ctx c.Context, dto *saveStepDTO) (uuid.UUID, time.Time, error) {
	storageData, errSerialize := json.Marshal(runCtx.VarStore)
	if errSerialize != nil {
		return uuid.Nil, time.Time{}, errSerialize
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
		Attachments: len(dto.attachments),
		CurrentExecutor: db.CurrentExecutorData{
			GroupID:       dto.currentExecutor.GroupID,
			GroupName:     dto.currentExecutor.GroupName,
			People:        dto.currentExecutor.People,
			InitialPeople: dto.currentExecutor.InitialPeople,
		},
	})
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
		},
	})
}

func ProcessBlockWithEndMapping(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext,
	manual bool,
) (bool, error) {
	ctx, s := trace.StartSpan(ctx, "process_block_with_end_mapping")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("workNumber", runCtx.WorkNumber)

	runCtx.BlockRunResults = &BlockRunResults{}

	blockProcessor := newBlockProcessor(name, bl, runCtx, manual)

	pErr := blockProcessor.ProcessBlock(ctx, 0)
	if pErr != nil {
		return false, pErr
	}

	updDeadlineErr := blockProcessor.updateTaskExecDeadline(ctx)
	if updDeadlineErr != nil {
		return false, updDeadlineErr
	}

	intStatus, stringStatus, err := runCtx.Services.Storage.GetTaskStatusWithReadableString(ctx, runCtx.TaskID)
	if err != nil {
		log.WithError(err).Error("couldn't get task status")

		return false, nil
	}

	if intStatus != 2 && intStatus != 4 {
		return false, nil
	}

	endErr := processBlockEnd(ctx, stringStatus, runCtx)
	if endErr != nil {
		log.WithError(endErr).Error("couldn't send process end notification")
	}

	return true, nil
}

func processBlockEnd(ctx c.Context, status string, runCtx *BlockRunContext) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_end")
	defer s.End()

	log := logger.GetLogger(ctx)

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

	req, err := http.NewRequestWithContext(ctx, settings.Method, settings.URL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

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
