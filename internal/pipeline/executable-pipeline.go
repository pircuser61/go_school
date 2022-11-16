package pipeline

import (
	c "context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v4"

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

func (gb *ExecutablePipeline) Members() map[string]struct{} {
	return nil
}

func (gb *ExecutablePipeline) CheckSLA() (bool, time.Time) {
	return false, time.Time{}
}

func (gb *ExecutablePipeline) GetStatus() Status {
	switch {
	case gb.IsOver():
		return StatusFinished
	case gb.ReadyToStart():
		return StatusReady
	case len(gb.ActiveBlocks) != 0:
		return StatusRunning
	default:
		return StatusIdle
	}
}

func (gb *ExecutablePipeline) UpdateManual() bool {
	return false
}

func (gb *ExecutablePipeline) IsOver() bool {
	return len(gb.ActiveBlocks) == 0
}

func (gb *ExecutablePipeline) ReadyToStart() bool {
	return len(gb.ActiveBlocks) == 0 && gb.EntryPoint == BlockGoFirstStart
}

func (gb *ExecutablePipeline) GetTaskHumanStatus() TaskHumanStatus {
	// TODO: проверять, что нет ошибок (потому что только тогда мы Done)
	if len(gb.ActiveBlocks) == 0 {
		return StatusDone
	}
	return StatusNew
}

type CreateTaskDTO struct {
	Author     string
	IsDebug    bool
	Params     []byte
	WorkNumber string
	RunCtx     entity.TaskRunContext
}

func (gb *ExecutablePipeline) CreateTask(ctx c.Context, tx pgx.Tx, dto *CreateTaskDTO) error {
	gb.TaskID = uuid.New()

	task, err := gb.Storage.CreateTask(ctx, tx, &db.CreateTaskDTO{
		TaskID:     gb.TaskID,
		VersionID:  gb.VersionID,
		Author:     dto.Author,
		WorkNumber: dto.WorkNumber,
		IsDebug:    dto.IsDebug,
		Params:     dto.Params,
		RunCtx:     dto.RunCtx,
	})
	if err != nil {
		return err
	}

	gb.WorkNumber = task.WorkNumber
	return nil
}

type stepCtx struct {
	workNumber  string
	workTitle   string
	description string
	stepStart   time.Time
}

func (gb *ExecutablePipeline) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *ExecutablePipeline) GetState() interface{} {
	return nil
}

func (gb *ExecutablePipeline) Update(_ c.Context) (interface{}, error) {
	return nil, nil
}

func (gb *ExecutablePipeline) CreateBlocks(ctx c.Context, source map[string]entity.EriusFunc) error {
	gb.Blocks = make(map[string]Runner)

	ctx, s := trace.StartSpan(ctx, "create_blocks")
	defer s.End()

	for k := range source {
		bl := source[k]

		block, err := CreateBlock(ctx, k, &bl, &BlockRunContext{
			TaskID:      gb.TaskID,
			WorkNumber:  gb.WorkNumber,
			WorkTitle:   gb.Name,
			Initiator:   gb.RunContext.Initiator,
			Storage:     gb.Storage,
			Sender:      gb.Sender,
			People:      gb.People,
			ServiceDesc: gb.ServiceDesc,
			FaaS:        gb.FaaS,
			VarStore:    gb.VarStore,
			UpdateData:  nil,
		})
		if err != nil {
			return err
		}

		gb.Blocks[k] = block
	}

	return nil
}

func getWorkIdKey(stepName string) string {
	return stepName + "." + keyStepWorkId
}
