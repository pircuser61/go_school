package pipeline

import (
	c "context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

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

	RunContext *BlockRunContext
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

func (ep *ExecutablePipeline) UpdateManual() bool {
	return false
}

func (ep *ExecutablePipeline) IsOver() bool {
	return len(ep.ActiveBlocks) == 0
}

func (ep *ExecutablePipeline) ReadyToStart() bool {
	return len(ep.ActiveBlocks) == 0 && ep.EntryPoint == BlockGoFirstStart
}

func (ep *ExecutablePipeline) GetTaskHumanStatus() TaskHumanStatus {
	// TODO: проверять, что нет ошибок (потому что только тогда мы Done)
	if len(ep.ActiveBlocks) == 0 {
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
	RunCtx     entity.TaskRunContext
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
		RunCtx:     dto.RunCtx,
	})
	if err != nil {
		return err
	}

	ep.WorkNumber = task.WorkNumber
	return nil
}

func (ep *ExecutablePipeline) Run(_ c.Context, _ *store.VariableStore) error {
	return nil
}

type stepCtx struct {
	workNumber  string
	workTitle   string
	description string
	stepStart   time.Time
}

//nolint:gocognit,gocyclo //its really complex
func (ep *ExecutablePipeline) DebugRun(_ c.Context, _ *stepCtx, _ *store.VariableStore) error {
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

func (ep *ExecutablePipeline) Update(_ c.Context) (interface{}, error) {
	return nil, nil
}

func (ep *ExecutablePipeline) CreateBlocks(ctx c.Context, source map[string]entity.EriusFunc) error {
	ep.Blocks = make(map[string]Runner)

	ctx, s := trace.StartSpan(ctx, "create_blocks")
	defer s.End()

	for k := range source {
		bl := source[k]

		block, err := CreateBlock(ctx, k, &bl, &BlockRunContext{
			TaskID:      ep.TaskID,
			WorkNumber:  ep.WorkNumber,
			WorkTitle:   ep.Name,
			Initiator:   ep.RunContext.Initiator,
			Storage:     ep.Storage,
			Sender:      ep.Sender,
			People:      ep.People,
			ServiceDesc: ep.ServiceDesc,
			FaaS:        ep.FaaS,
			VarStore:    ep.VarStore,
			UpdateData:  nil,
		})
		if err != nil {
			return err
		}

		ep.Blocks[k] = block
	}

	return nil
}

func getWorkIdKey(stepName string) string {
	return stepName + "." + keyStepWorkId
}
