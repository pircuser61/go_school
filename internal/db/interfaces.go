package db

import (
	c "context"
	"time"

	"github.com/google/uuid"
	"github.com/iancoleman/orderedmap"
	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type DictionaryStorager interface {
	GetApproveActionNames(ctx c.Context) ([]e.ApproveActionName, error)
	GetApproveStatuses(ctx c.Context) ([]e.ApproveStatus, error)
}

type PipelineStorager interface {
	GetWorkedVersions(ctx c.Context) ([]e.EriusScenario, error)
	GetPipeline(ctx c.Context, id uuid.UUID) (*e.EriusScenario, error)

	CreatePipeline(c c.Context, p *e.EriusScenario, author string, pipelineData []byte) error
	PipelineRemovable(ctx c.Context, id uuid.UUID) (bool, error)
	DeletePipeline(ctx c.Context, id uuid.UUID) error
	RenamePipeline(ctx c.Context, id uuid.UUID, name string) error
}

type TaskStorager interface {
	GetTaskFormSchemaID(workNumber, formID string) (string, error)
	GetAdditionalForms(workNumber, nodeName string) ([]string, error)
	GetApplicationData(workNumber string) (*orderedmap.OrderedMap, error)
	SetApplicationData(workNumber string, data *orderedmap.OrderedMap) error
	GetTasks(ctx c.Context, filters e.TaskFilter, delegations []string) (*e.EriusTasksPage, error)
	GetTasksCount(ctx c.Context, currentUser string, delegationsByApprovement, delegationsByExecution []string) (*e.CountTasks, error)
	GetPipelineTasks(ctx c.Context, pipelineID uuid.UUID) (*e.EriusTasks, error)
	GetTask(ctx c.Context, delegationsApprover, delegationsExecution []string, currentUser, workNumber string) (*e.EriusTask, error)
	GetTaskSteps(ctx c.Context, id uuid.UUID) (e.TaskSteps, error)
	GetUnfinishedTaskStepsByWorkIdAndStepType(ctx c.Context, id uuid.UUID, stepType string) (e.TaskSteps, error)
	GetTaskStepById(ctx c.Context, id uuid.UUID) (*e.Step, error)
	GetParentTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetVersionTasks(ctx c.Context, versionID uuid.UUID) (*e.EriusTasks, error)
	GetLastDebugTask(ctx c.Context, versionID uuid.UUID, author string) (*e.EriusTask, error)
	GetUsersWithReadWriteFormAccess(ctx c.Context, workNumber, stepName string) ([]e.UsersWithFormAccess, error)

	CreateTask(ctx c.Context, dto *CreateTaskDTO) (*e.EriusTask, error)
	UpdateTaskStatus(ctx c.Context, taskID uuid.UUID, status int) error
	GetTaskStatus(ctx c.Context, taskID uuid.UUID) (int, error)
	StopTaskBlocks(ctx c.Context, taskID uuid.UUID) error
	UpdateTaskHumanStatus(ctx c.Context, taskID uuid.UUID, status string) error
	CheckTaskStepsExecuted(ctx c.Context, workNumber string, blocks []string) (bool, error)
	GetTaskStepsToWait(ctx c.Context, workNumber, blockName string) ([]string, error)
	CheckUserCanEditForm(ctx c.Context, workNumber string, stepName string, login string) (bool, error)
	GetTaskRunContext(ctx c.Context, workNumber string) (e.TaskRunContext, error)
	GetBlockDataFromVersion(ctx c.Context, workNumber, blockName string) (*e.EriusFunc, error)
	GetVariableStorageForStep(ctx c.Context, taskID uuid.UUID, stepType string) (*store.VariableStore, error)
	GetBlocksBreachedSLA(ctx c.Context) ([]StepBreachedSLA, error)
	UpdateTaskRate(ctx c.Context, req *UpdateTaskRate) error
	GetMeanTaskSolveTime(ctx c.Context, pipelineId string) ([]e.TaskCompletionInterval, error)
	SendTaskToArchive(ctx c.Context, taskID uuid.UUID) (err error)
	CheckIsArchived(ctx c.Context, taskID uuid.UUID) (bool, error)

	GetTaskForMonitoring(ctx c.Context, workNumber string) ([]e.MonitoringTaskNode, error)
}

type UpdateTaskRate struct {
	ByLogin    string
	WorkNumber string
	Comment    *string
	Rate       *int
}

type DbMemberAction struct {
	Id   string
	Type string
}

type DbMember struct {
	Login    string
	Finished bool
	Actions  []DbMemberAction
}

type DbDeadline struct {
	Deadline time.Time
	Action   string
}

type SaveStepRequest struct {
	WorkID      uuid.UUID
	StepType    string
	StepName    string
	Content     []byte
	BreakPoints []string
	HasError    bool
	Status      string
	Members     []DbMember
	Deadlines   []DbDeadline
}

type UpdateStepRequest struct {
	Id          uuid.UUID
	StepName    string
	Content     []byte
	BreakPoints []string
	HasError    bool
	Status      string
	Members     []DbMember
	Deadlines   []DbDeadline
}

type UpdateTaskBlocksDataRequest struct {
	Id                     uuid.UUID
	ActiveBlocks           map[string]struct{}
	SkippedBlocks          map[string]struct{}
	NotifiedBlocks         map[string][]string
	PrevUpdateStatusBlocks map[string]string
}

type SearchPipelineRequest struct {
	PipelineId   *string
	PipelineName *string
	Limit        int
	Offset       int
}

type StepBreachedSLA struct {
	TaskID     uuid.UUID
	WorkNumber string
	WorkTitle  string
	Initiator  string
	VarStore   *store.VariableStore
	BlockData  *e.EriusFunc
	StepName   string
	Action     e.TaskUpdateAction
}

//go:generate mockery --name=Database --structname=MockedDatabase
type Database interface {
	PipelineStorager
	TaskStorager
	DictionaryStorager

	Ping(ctx c.Context) error

	StartTransaction(ctx c.Context) (Database, error)
	CommitTransaction(ctx c.Context) error
	RollbackTransaction(ctx c.Context) error

	GetPipelinesWithLatestVersion(ctx c.Context, author string) ([]e.EriusScenarioInfo, error)
	GetApprovedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	GetVersionsByStatus(ctx c.Context, status int, author string) ([]e.EriusScenarioInfo, error)
	GetDraftVersions(ctx c.Context, author string) ([]e.EriusScenarioInfo, error)
	GetOnApproveVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	SwitchApproved(ctx c.Context, pipelineID, versionID uuid.UUID, author string) error
	VersionEditable(ctx c.Context, versionID uuid.UUID) (bool, error)
	CreateVersion(ctx c.Context, p *e.EriusScenario, author string, pipelineData []byte) error
	DeleteVersion(ctx c.Context, versionID uuid.UUID) error
	GetPipelineVersion(ctx c.Context, id uuid.UUID, checkNotDeleted bool) (*e.EriusScenario, error)
	GetPipelineVersions(ctx c.Context, id uuid.UUID) ([]e.EriusVersionInfo, error)
	UpdateDraft(ctx c.Context, p *e.EriusScenario, pipelineData []byte) error
	SaveStepContext(ctx c.Context, dto *SaveStepRequest) (uuid.UUID, time.Time, error)
	UpdateStepContext(ctx c.Context, dto *UpdateStepRequest) error
	UpdateTaskBlocksData(ctx c.Context, dto *UpdateTaskBlocksDataRequest) error

	GetExecutableScenarios(ctx c.Context) ([]e.EriusScenario, error)
	GetExecutableByName(ctx c.Context, name string) (*e.EriusScenario, error)

	ActiveAlertNGSA(ctx c.Context, sever int,
		state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error
	ClearAlertNGSA(ctx c.Context, name string) error
	CreateTag(ctx c.Context, e *e.EriusTagInfo, author string) (*e.EriusTagInfo, error)
	GetTag(ctx c.Context, e *e.EriusTagInfo) (*e.EriusTagInfo, error)
	EditTag(ctx c.Context, e *e.EriusTagInfo) error
	RemoveTag(ctx c.Context, id uuid.UUID) error
	GetAllTags(ctx c.Context) ([]e.EriusTagInfo, error)
	GetPipelineTag(ctx c.Context, id uuid.UUID) ([]e.EriusTagInfo, error)
	AttachTag(ctx c.Context, id uuid.UUID, p *e.EriusTagInfo) error
	DetachTag(ctx c.Context, id uuid.UUID, p *e.EriusTagInfo) error
	RemovePipelineTags(ctx c.Context, id uuid.UUID) error
	PipelineNameCreatable(ctx c.Context, name string) (bool, error)
	SwitchRejected(ctx c.Context, versionID uuid.UUID, comment, author string) error
	GetRejectedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	RollbackVersion(ctx c.Context, pipelineID, versionID uuid.UUID) error
	GetVersionsByPipelineID(ctx c.Context, blueprintID string) ([]e.EriusScenario, error)
	GetVersionByWorkNumber(ctx c.Context, workNumber string) (*e.EriusScenario, error)
	GetPipelinesByNameOrId(ctx c.Context, dto *SearchPipelineRequest) ([]e.SearchPipeline, error)

	GetBlocksOutputs(ctx c.Context, blockId string) (e.BlockOutputs, error)
	GetBlockOutputs(ctx c.Context, blockId, blockName string) (e.BlockOutputs, error)
	GetBlockInputs(ctx c.Context, blockName, workNumber string) (e.BlockInputs, error)
	GetMergedVariableStorage(ctx c.Context, workId uuid.UUID, blockIds []string) (*store.VariableStore, error)
	GetTasksForMonitoring(ctx c.Context, filters e.TasksForMonitoringFilters) (*e.TasksForMonitoring, error)
}
