package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

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
	NodeEvents []entity.NodeEvent
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
	}
	return &runCtxCopy
}

//nolint:gocyclo //todo: need to decompose
func processBlock(ctx c.Context, name string, its int, bl *entity.EriusFunc, runCtx *BlockRunContext, manual bool) (err error) {
	its++
	if its > 10 {
		return errors.New("took too long")
	}

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

	statusBeforeUpdate := block.GetStatus()

	if (block.UpdateManual() && manual) || !block.UpdateManual() {
		err = updateBlock(ctx, block, name, id, runCtx)
		if err != nil {
			return
		}
	}

	taskHumanStatus, statusComment, action := block.GetTaskHumanStatus()
	err = runCtx.updateStatusByStep(ctx, taskHumanStatus, statusComment)
	if err != nil {
		return err
	}

	newEvents := block.GetNewEvents()
	runCtx.BlockRunResults.NodeEvents = append(runCtx.BlockRunResults.NodeEvents, newEvents...)

	isArchived, err := runCtx.Services.Storage.CheckIsArchived(ctx, runCtx.TaskID)
	if err != nil {
		return err
	}

	if isArchived || (block.GetStatus() != StatusFinished &&
		block.GetStatus() != StatusNoSuccess &&
		block.GetStatus() != StatusError) ||
		((runCtx.UpdateData != nil) && (statusBeforeUpdate == block.GetStatus())) {
		return nil
	}

	err = runCtx.handleInitiatorNotify(ctx, handleInitiatorNotifyParams{
		step:     name,
		stepType: bl.TypeID,
		action:   action,
		status:   taskHumanStatus,
	})
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
		if err = processBlock(ctx, b, its, blockData, ctxCopy, false); err != nil {
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

		err = epi.Storage.SetLastRunID(ctx, runCtx.TaskID, epi.VersionID)
		if err != nil {
			return nil, false, errors.Wrap(err, "can’t set id of the last runned task")
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
			Login:                pl[i].Login,
			Actions:              actions,
			IsActed:              pl[i].IsActed,
			ExecutionGroupMember: pl[i].ExecutionGroupMember,
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
			Login:                pl[i].Login,
			Actions:              actions,
			IsActed:              pl[i].IsActed,
			ExecutionGroupMember: pl[i].ExecutionGroupMember,
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

func (runCtx *BlockRunContext) getFileField() ([]string, error) {
	task, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, err
	}

	return task.InitialApplication.AttachmentFields, nil
}

func (runCtx *BlockRunContext) makeNotificationFormAttachment(files []string) ([]file_registry.FileInfo, error) {
	attachments := make([]entity.Attachment, 0)
	mapFiles := make(map[string][]entity.Attachment)
	for _, v := range files {
		attachments = append(attachments, entity.Attachment{FileID: v})
	}

	mapFiles["files"] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(c.Background(), mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]file_registry.FileInfo, 0)
	for _, v := range file["files"] {
		ta = append(ta, file_registry.FileInfo{FileId: v.FileId, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

func (runCtx *BlockRunContext) makeNotificationAttachment() ([]file_registry.FileInfo, error) {
	task, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, err
	}

	attachments := make([]entity.Attachment, 0)
	mapFiles := make(map[string][]entity.Attachment)
	for _, v := range task.InitialApplication.AttachmentFields {
		filesAttach, ok := task.InitialApplication.ApplicationBody.Get(v)
		if ok {
			switch data := filesAttach.(type) {
			case orderedmap.OrderedMap:
				fileId, get := data.Get("file_id")
				if !get {
					continue
				}

				attachments = append(attachments, entity.Attachment{FileID: fileId.(string)})
			case []interface{}:
				for _, vv := range data {
					fileMap := vv.(orderedmap.OrderedMap)
					fileId, oks := fileMap.Get("file_id")
					if !oks {
						continue
					}

					attachments = append(attachments, entity.Attachment{FileID: fileId.(string)})
				}
			}
		}
	}

	mapFiles["files"] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(c.Background(), mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]file_registry.FileInfo, 0)
	for _, v := range file["files"] {
		ta = append(ta, file_registry.FileInfo{FileId: v.FileId, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

func (runCtx *BlockRunContext) makeNotificationDescription(nodeName string) ([]orderedmap.OrderedMap, []e.Attachment, error) {
	descr, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, nil, err
	}

	apBody := flatArray(descr.InitialApplication.ApplicationBody)

	descriptions := make([]orderedmap.OrderedMap, 0)

	filesAttach, err := runCtx.makeNotificationAttachment()
	if err != nil {
		return nil, nil, err
	}

	attachments, err := runCtx.GetAttach(filesAttach)
	if err != nil {
		return nil, nil, err
	}

	files := make([]e.Attachment, 0, len(attachments.AttachmentsList))

	if len(apBody.Values()) != 0 {
		apBody.Set("attachLinks", attachments.AttachLinks)
		apBody.Set("attachExist", attachments.AttachExists)
		apBody.Set("attachList", attachments.AttachmentsList)
	}

	descriptions = append(descriptions, apBody)

	additionalForms, err := runCtx.Services.Storage.GetAdditionalDescriptionForms(runCtx.WorkNumber, nodeName)
	if err != nil {
		return nil, nil, err
	}

	for _, v := range additionalForms {
		attachmentFiles := make([]string, 0)

		for _, val := range v.Values() {
			file, ok := val.(orderedmap.OrderedMap)
			if !ok {
				continue
			}

			if fileId, fileOk := file.Get("file_id"); fileOk {
				attachmentFiles = append(attachmentFiles, fileId.(string))
			}
		}

		fileInfo, fileErr := runCtx.makeNotificationFormAttachment(attachmentFiles)
		if fileErr != nil {
			return nil, nil, err
		}

		attach, attachErr := runCtx.GetAttach(fileInfo)
		if attachErr != nil {
			return nil, nil, err
		}

		v.Set("attachLinks", attach.AttachLinks)
		v.Set("attachExist", attach.AttachExists)
		v.Set("attachList", attach.AttachmentsList)

		files = append(files, attach.AttachmentsList...)
		descriptions = append(descriptions, flatArray(v))
	}

	files = append(files, attachments.AttachmentsList...)
	return descriptions, files, nil
}

func flatArray(v orderedmap.OrderedMap) orderedmap.OrderedMap {
	res := orderedmap.New()
	keys := v.Keys()
	values := v.Values()

	for _, k := range keys {
		vv, ok := values[k].([]interface{})
		if ok {
			for i, v := range vv {
				res.Set(k+"("+strconv.Itoa(i)+")", v)
			}
		} else {
			res.Set(k, values[k])
		}
	}

	return *res
}

type handleInitiatorNotifyParams struct {
	step     string
	stepType string
	action   string
	status   TaskHumanStatus
}

func (runCtx *BlockRunContext) handleInitiatorNotify(ctx c.Context, params handleInitiatorNotifyParams) error {
	const (
		FormStepType     = "form"
		TimerStepType    = "timer"
		FunctionStepType = "executable_function"
	)

	if runCtx.skipNotifications {
		return nil
	}

	switch params.status {
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

	if params.status == StatusDone && (params.stepType == FormStepType || params.stepType == FunctionStepType || params.stepType == TimerStepType) {
		return nil
	}

	description, files, err := runCtx.makeNotificationDescription(params.step)
	if err != nil {
		return err
	}

	loginsToNotify := []string{runCtx.Initiator}

	log := logger.GetLogger(ctx)

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = runCtx.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithField("login", login).WithError(err).Warning("couldn't get email")
			return nil
		}

		emails = append(emails, email)
	}

	if params.action == "" {
		params.action = statusToTaskState[params.status]
	}

	tmpl := mail.NewAppInitiatorStatusNotificationTpl(
		&mail.SignerNotifTemplate{
			WorkNumber:  runCtx.WorkNumber,
			Name:        runCtx.NotifName,
			SdUrl:       runCtx.Services.Sender.SdAddress,
			Description: description,
			Action:      params.action,
		})

	iconsName := []string{tmpl.Image}

	for _, v := range description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				descIcons := []string{downloadImg}
				iconsName = append(iconsName, descIcons...)
				break
			}
		}
	}

	iconFiles, iconErr := runCtx.GetIcons(iconsName)
	if iconErr != nil {
		return err
	}

	files = append(files, iconFiles...)

	if sendErr := runCtx.Services.Sender.SendNotification(ctx, emails, files, tmpl); sendErr != nil {
		return sendErr
	}

	return nil
}

func ProcessBlockWithEndMapping(ctx c.Context, name string, bl *entity.EriusFunc, runCtx *BlockRunContext,
	manual bool) error {
	ctx, s := trace.StartSpan(ctx, "process_block_with_end_mapping")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("workNumber", runCtx.WorkNumber)

	runCtx.BlockRunResults = &BlockRunResults{}

	pErr := processBlock(ctx, name, 0, bl, runCtx, manual)
	if pErr != nil {
		return pErr
	}
	intStatus, stringStatus, err := runCtx.Services.Storage.GetTaskStatusWithReadableString(ctx, runCtx.TaskID)
	if err != nil {
		log.WithError(err).Error("couldn't get task status")
		return nil
	}

	if intStatus != 2 && intStatus != 4 {
		return nil
	}

	endErr := processBlockEnd(ctx, stringStatus, runCtx)
	if endErr != nil {
		log.WithError(endErr).Error("couldn't send process end notification")
	}
	return nil
}

func processBlockEnd(ctx c.Context, status string, runCtx *BlockRunContext) (err error) {
	ctx, s := trace.StartSpan(ctx, "process_block_end")
	defer s.End()

	log := logger.GetLogger(ctx)

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
			systemSettings.OutputSettings.MicroserviceId == "" {
			log.Info(fmt.Sprintf("no output settings for clientID %s", context.ClientID))
			return nil
		}
		taskTime, timeErr := runCtx.Services.Storage.GetTaskInWorkTime(ctx, runCtx.WorkNumber)
		if timeErr != nil {
			return timeErr
		}
		sendingErr := sendEndingMapping(ctx, &entity.EndProcessData{
			Id:         runCtx.TaskID.String(),
			VersionId:  version.VersionID.String(),
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

func sendEndingMapping(ctx c.Context, data *entity.EndProcessData,
	runCtx *BlockRunContext, settings *entity.EndSystemSettings) (err error) {
	secretsHumanKey, secretsErr := runCtx.Services.Integrations.GetMicroserviceHumanKey(
		ctx,
		settings.MicroserviceId,
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
