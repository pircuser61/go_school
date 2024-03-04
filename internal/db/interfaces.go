package db

import (
	c "context"
	"time"

	"github.com/google/uuid"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type DictionaryStorager interface {
	GetApproveActionNames(ctx c.Context) ([]e.ApproveActionName, error)
	GetApproveStatuses(ctx c.Context) ([]e.ApproveStatus, error)
	GetNodeDecisions(ctx c.Context) ([]e.NodeDecision, error)
}

type PipelineStorager interface {
	GetWorkedVersions(ctx c.Context) ([]e.EriusScenario, error)
	GetPipeline(ctx c.Context, id uuid.UUID) (*e.EriusScenario, error)

	CreatePipeline(c c.Context, p *e.EriusScenario, author string, pipelineData []byte, oldVersionID uuid.UUID, hasPrivateFunction bool) error
	PipelineRemovable(ctx c.Context, id uuid.UUID) (bool, error)
	DeletePipeline(ctx c.Context, id uuid.UUID) error
	RenamePipeline(ctx c.Context, id uuid.UUID, name string) error
}

type TaskStorager interface {
	GetTaskFormSchemaID(workNumber, formID string) (string, error)
	GetAdditionalDescriptionForms(workNumber, nodeName string) ([]e.DescriptionForm, error)
	GetAdditionalForms(workNumber, nodeName string) ([]string, error)
	GetApplicationData(workNumber string) (string, error)
	GetDeadline(ctx c.Context, workID string) (time.Time, error)
	GetTasks(ctx c.Context, filters e.TaskFilter, delegations []string) (*e.EriusTasksPage, error)
	GetTasksCount(ctx c.Context, currentUser string, delegationsByApprovement, delegationsByExecution []string) (*e.CountTasks, error)
	GetPipelineTasks(ctx c.Context, pipelineID uuid.UUID) (*e.EriusTasks, error)
	GetTask(ctx c.Context, delegationsApprover, delegationsExecution []string, currentUser, workNumber string) (*e.EriusTask, error)
	GetTaskSteps(ctx c.Context, id uuid.UUID) (e.TaskSteps, error)
	GetUnfinishedTaskStepsByWorkIDAndStepType(ctx c.Context, id uuid.UUID, stepType string, in *e.TaskUpdate) (e.TaskSteps, error)
	GetTaskStepByID(ctx c.Context, id uuid.UUID) (*e.Step, error)
	GetParentTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetCanceledTaskSteps(ctx c.Context, taskID uuid.UUID) ([]e.Step, error)
	GetVersionTasks(ctx c.Context, versionID uuid.UUID) (*e.EriusTasks, error)
	GetLastDebugTask(ctx c.Context, versionID uuid.UUID, author string) (*e.EriusTask, error)
	GetTaskStatus(ctx c.Context, taskID uuid.UUID) (int, error)
	GetTaskHumanStatus(ctx c.Context, taskID uuid.UUID) (string, error)
	GetTaskStatusWithReadableString(ctx c.Context, taskID uuid.UUID) (int, string, error)
	GetTaskStepsToWait(ctx c.Context, workNumber, blockName string) ([]string, error)
	GetTaskRunContext(ctx c.Context, workNumber string) (e.TaskRunContext, error)
	GetBlockDataFromVersion(ctx c.Context, workNumber, blockName string) (*e.EriusFunc, error)
	GetVariableStorageForStep(ctx c.Context, taskID uuid.UUID, stepType string) (*store.VariableStore, error)
	GetVariableStorage(ctx c.Context, workNumber string) (*store.VariableStore, error)
	GetBlocksBreachedSLA(ctx c.Context) ([]StepBreachedSLA, error)
	GetMeanTaskSolveTime(ctx c.Context, pipelineID string) ([]e.TaskCompletionInterval, error)
	GetTaskInWorkTime(ctx c.Context, workNumber string) (*e.TaskCompletionInterval, error)
	GetExecutorsFromPrevExecutionBlockRun(ctx c.Context, taskID uuid.UUID, name string) (exec map[string]struct{}, err error)
	GetExecutorsFromPrevWorkVersionExecutionBlockRun(ctx c.Context, workNumber, name string) (exec map[string]struct{}, err error)
	GetTaskForMonitoring(ctx c.Context, workNumber string) ([]e.MonitoringTaskNode, error)
	GetWorkIDByWorkNumber(ctx c.Context, workNumber string) (uuid.UUID, error)

	CreateTask(ctx c.Context, dto *CreateTaskDTO) (*e.EriusTask, error)

	CheckUserCanEditForm(ctx c.Context, workNumber string, stepName string, login string) (bool, error)
	SendTaskToArchive(ctx c.Context, taskID uuid.UUID) (err error)
	CheckIsArchived(ctx c.Context, taskID uuid.UUID) (bool, error)
	GetTaskCustomProps(ctx c.Context, taskID uuid.UUID) (*TaskCustomProps, error)
	StopTaskBlocks(ctx c.Context, taskID uuid.UUID) error
	FinishTaskBlocks(ctx c.Context, workID uuid.UUID, ignoreSteps []string, updateParent bool) error
	ParallelIsFinished(ctx c.Context, workNumber, blockName string) (bool, error)
	UnsetIsActive(ctx c.Context, workNumber, blockName string) error

	UpdateTaskRate(ctx c.Context, req *UpdateTaskRate) error
	UpdateTaskHumanStatus(ctx c.Context, taskID uuid.UUID, status, comment string) (*e.EriusTask, error)
	UpdateTaskStatus(ctx c.Context, taskID uuid.UUID, status int, comment, author string) error
	UpdateBlockStateInOthers(ctx c.Context, blockName, taskID string, blockState []byte) error
	UpdateBlockVariablesInOthers(ctx c.Context, taskID string, values map[string]interface{}) error
}

type UpdateTaskRate struct {
	ByLogin    string
	WorkNumber string
	Comment    *string
	Rate       *int
}

type MemberAction struct {
	ID     string
	Type   string
	Params map[string]interface{}
}

type TaskAction struct {
	BlockID     string                            `json:"block_id"`
	Actions     []string                          `json:"actions"`
	Params      map[string]map[string]interface{} `json:"params"`
	IsInitiator bool                              `json:"is_initiator"`
}

type Member struct {
	Login                string
	Actions              []MemberAction
	Type                 string
	IsActed              bool
	Finished             bool
	ExecutionGroupMember bool
	IsInitiator          bool
}

type Deadline struct {
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
	Members     []Member
	Deadlines   []Deadline
	IsReEntry   bool
}

type UpdateStepRequest struct {
	ID          uuid.UUID
	StepName    string
	Content     []byte
	BreakPoints []string
	HasError    bool
	Status      string
	Members     []Member
	Deadlines   []Deadline
}

type UpdateTaskBlocksDataRequest struct {
	ID                     uuid.UUID
	ActiveBlocks           map[string]struct{}
	SkippedBlocks          map[string]struct{}
	NotifiedBlocks         map[string][]string
	PrevUpdateStatusBlocks map[string]string
}

type SearchPipelineRequest struct {
	PipelineID   *string
	PipelineName *string
	Limit        int
	Offset       int
}

type StepBreachedSLA struct {
	TaskID      uuid.UUID
	WorkNumber  string
	WorkTitle   string
	Initiator   string
	VarStore    *store.VariableStore
	BlockData   *e.EriusFunc
	StepName    string
	Action      e.TaskUpdateAction
	IsTest      bool
	CustomTitle string
}

//go:generate mockery --name=Database --structname=MockedDatabase --with-expecter
type Database interface {
	PipelineStorager
	TaskStorager
	DictionaryStorager

	Ping(ctx c.Context) error

	Acquire(ctx c.Context) (Database, error)
	Release(ctx c.Context) error

	StartTransaction(ctx c.Context) (Database, error)
	CommitTransaction(ctx c.Context) error
	RollbackTransaction(ctx c.Context) error

	GetPipelinesWithLatestVersion(
		ctx c.Context,
		authorLogin string,
		published bool,
		page, perPage *int,
		filter string,
	) ([]e.EriusScenarioInfo, error)
	GetApprovedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	GetVersionsByStatus(ctx c.Context, status int, author string) ([]e.EriusScenarioInfo, error)
	GetDraftVersions(ctx c.Context, author string) ([]e.EriusScenarioInfo, error)
	GetOnApproveVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	SwitchApproved(ctx c.Context, pipelineID, versionID uuid.UUID, author string) error
	VersionEditable(ctx c.Context, versionID uuid.UUID) (bool, error)
	CreateVersion(
		ctx c.Context,
		p *e.EriusScenario,
		author string,
		pipelineData []byte,
		oldVersionID uuid.UUID,
		hasPrivateFunction bool,
	) error
	DeleteVersion(ctx c.Context, versionID uuid.UUID) error
	GetPipelineVersion(ctx c.Context, id uuid.UUID, checkNotDeleted bool) (*e.EriusScenario, error)
	GetPipelineVersions(ctx c.Context, id uuid.UUID) ([]e.EriusVersionInfo, error)
	UpdateDraft(ctx c.Context, p *e.EriusScenario, pipelineData []byte, groups []*e.NodeGroup, isHidden bool) error
	SaveStepContext(ctx c.Context, dto *SaveStepRequest) (uuid.UUID, time.Time, error)
	UpdateStepContext(ctx c.Context, dto *UpdateStepRequest) error
	UpdateTaskBlocksData(ctx c.Context, dto *UpdateTaskBlocksDataRequest) error

	GetExecutableScenarios(ctx c.Context) ([]e.EriusScenario, error)
	GetExecutableByName(ctx c.Context, name string) (*e.EriusScenario, error)

	SetLastRunID(ctx c.Context, taskID, versionID uuid.UUID) error
	SwitchRejected(ctx c.Context, versionID uuid.UUID, comment, author string) error
	GetRejectedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	RollbackVersion(ctx c.Context, pipelineID, versionID uuid.UUID) error
	GetVersionByPipelineID(ctx c.Context, pipelineID string) (*e.EriusScenario, error)
	GetVersionByWorkNumber(ctx c.Context, workNumber string) (*e.EriusScenario, error)
	GetPipelinesByNameOrID(ctx c.Context, dto *SearchPipelineRequest) ([]e.SearchPipeline, error)
	GetVersionsByFunction(ctx c.Context, functionID, versionID string) ([]e.EriusScenario, error)

	GetBlocksOutputs(ctx c.Context, blockID string) (e.BlockOutputs, error)
	GetBlockOutputs(ctx c.Context, blockID, blockName string) (e.BlockOutputs, error)
	GetBlockInputs(ctx c.Context, blockName, workNumber string) (e.BlockInputs, error)
	CheckBlockForHiddenFlag(ctx c.Context, blockID string) (bool, error)
	GetMergedVariableStorage(ctx c.Context, workID uuid.UUID, blockIds []string) (*store.VariableStore, error)
	CheckTaskForHiddenFlag(ctx c.Context, workNumber string) (bool, error)
	GetTasksForMonitoring(ctx c.Context, filters *e.TasksForMonitoringFilters) (*e.TasksForMonitoring, error)
	GetBlockState(ctx c.Context, blockID string) (e.BlockState, error)

	SaveVersionSettings(ctx c.Context, settings e.ProcessSettings, schemaFlag *string) error
	SaveVersionMainSettings(ctx c.Context, settings e.ProcessSettings) error
	GetVersionSettings(ctx c.Context, versionID string) (e.ProcessSettings, error)
	AddExternalSystemToVersion(ctx c.Context, versionID string, systemID string) error
	GetExternalSystemsIDs(ctx c.Context, versionID string) ([]uuid.UUID, error)
	GetExternalSystemSettings(ctx c.Context, versionID string, systemID string) (e.ExternalSystem, error)
	GetExternalSystemTaskSubscriptions(ctx c.Context, versionID string, systemID string) (e.ExternalSystemSubscriptionParams, error)
	GetTaskEventsParamsByWorkNumber(ctx c.Context, workNumber string, systemID string) (e.ExternalSystemSubscriptionParams, error)
	RemoveExternalSystem(ctx c.Context, versionID string, systemID string) error
	RemoveExternalSystemTaskSubscriptions(ctx c.Context, versionID string, systemID string) error
	SaveExternalSystemSettings(ctx c.Context, versionID string, settings e.ExternalSystem, schemaFlag *string) error
	SaveExternalSystemSubscriptionParams(ctx c.Context, versionID string, params *e.ExternalSystemSubscriptionParams) error
	RemoveObsoleteMapping(ctx c.Context, id string) error
	GetWorksForUserWithGivenTimeRange(ctx c.Context, hours int, login, vID, workNumber string) ([]*e.EriusTask, error)
	CheckPipelineNameExists(c.Context, string, bool) (*bool, error)
	UpdateEndingSystemSettings(ctx c.Context, versionID, systemID string, settings e.EndSystemSettings) (err error)
	AllowRunAsOthers(ctx c.Context, versionID, systemID string, allowRunAsOthers bool) error
	SaveSLAVersionSettings(ctx c.Context, versionID string, s e.SLAVersionSettings) (err error)
	GetSLAVersionSettings(ctx c.Context, versionID string) (s e.SLAVersionSettings, err error)
	GetTaskMembers(ctx c.Context, workNumber string, fromActiveNodes bool) ([]Member, error)
	UpdateGroupsForEmptyVersions(ctx c.Context, versionID string, groups []*e.NodeGroup) error
	GetApprovalListSettings(ctx c.Context, listID string) (*e.ApprovalListSettings, error)
	GetApprovalListsSettings(ctx c.Context, versionID string) ([]e.ApprovalListSettings, error)
	SaveApprovalListSettings(ctx c.Context, in e.SaveApprovalListSettings) (id string, err error)
	UpdateApprovalListSettings(ctx c.Context, in e.UpdateApprovalListSettings) error
	RemoveApprovalListSettings(ctx c.Context, listID string) error
	GetFilteredStates(ctx c.Context, steps []string, wNumber string) (
		map[string]map[string]interface{},
		map[string]map[string]*time.Time,
		error,
	)
}
