package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const XRequestIDHeader = "X-Request-Id"

type initiation struct {
	db          db.Database
	faaS        string
	httpClient  *http.Client
	remedy      string
	sender      *mail.Service
	serviceDesc *servicedesc.Service
	peopleSrv   *people.Service
}

func NewInitiation(
	dbConn db.Database,
	faaS string,
	httpClient *http.Client,
	remedy string,
	sender *mail.Service,
	serviceDesc *servicedesc.Service,
	peopleSrv *people.Service,
) *initiation {
	return &initiation{
		db:          dbConn,
		faaS:        faaS,
		httpClient:  httpClient,
		remedy:      remedy,
		sender:      sender,
		serviceDesc: serviceDesc,
		peopleSrv:   peopleSrv,
	}
}

func (p *initiation) InitPipelines(ctx c.Context) error {
	log := logger.GetLogger(ctx)
	log.Info("--- init pipelines ---")

	var (
		success, failed int
	)

	unfinished, err := p.db.GetUnfinishedTasks(ctx)
	if err != nil {
		return err
	}

	if unfinished == nil {
		return errors.New("can`t init pipelines, empty tasks")
	}

	log.Info(fmt.Sprintf("--- init pipelines count: %d ---", len(unfinished.Tasks)))

	failedPipelinesCh := make(chan string, len(unfinished.Tasks))

	workers := 5
	if workers > len(unfinished.Tasks) {
		workers = 1
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	go p.poolWorkers(ctx, &wg, workers, unfinished.Tasks, failedPipelinesCh)
	wg.Wait()

	close(failedPipelinesCh)

	for range failedPipelinesCh {
		failed++
	}

	success = len(unfinished.Tasks) - failed

	log.Info(fmt.Sprintf("--- init pipelines finished with: success: %d, failed: %d---", success, failed))

	return nil
}

func (p *initiation) worker(ctx c.Context, wg *sync.WaitGroup, in chan entity.EriusTask, outCh chan string) {
	defer wg.Done()
	for {
		task, ok := <-in
		if !ok {
			return
		}

		log := logger.GetLogger(ctx)

		isFailed := false

		version, errVersion := p.db.GetVersionByWorkNumber(ctx, task.WorkNumber)
		if errVersion != nil {
			log.Error(errVersion)
			outCh <- task.WorkNumber
			continue
		}

		ep := ExecutablePipeline{}
		ep.WorkNumber = task.WorkNumber
		ep.TaskID = task.ID
		ep.PipelineID = version.ID
		ep.VersionID = task.VersionID
		ep.Storage = p.db
		ep.FaaS = p.faaS
		ep.PipelineModel = version
		ep.HTTPClient = p.httpClient
		ep.Remedy = p.remedy
		ep.ActiveBlocks = map[string]struct{}{}
		ep.SkippedBlocks = map[string]struct{}{}
		ep.EntryPoint = BlockGoFirstStart
		ep.Sender = p.sender
		ep.People = p.peopleSrv
		ep.Name = task.Name
		ep.Initiator = task.Author
		ep.ServiceDesc = p.serviceDesc
		ep.notifiedBlocks = map[string][]TaskHumanStatus{}
		ep.prevUpdateStatusBlocks = map[string]TaskHumanStatus{}

		if task.ActiveBlocks != nil {
			ep.ActiveBlocks = task.ActiveBlocks
		}

		if task.SkippedBlocks != nil {
			ep.SkippedBlocks = task.SkippedBlocks
		}

		if task.NotifiedBlocks != nil {
			notifiedBlocks := make(map[string][]TaskHumanStatus)

			for i := range task.NotifiedBlocks {
				for j := range task.NotifiedBlocks[i] {
					notifiedBlocks[i][j] = TaskHumanStatus(task.NotifiedBlocks[i][j])
				}
			}

			ep.notifiedBlocks = notifiedBlocks
		}

		if task.PrevUpdateStatusBlocks != nil {
			prevUpdateStatusBlocks := make(map[string]TaskHumanStatus)
			for i := range task.PrevUpdateStatusBlocks {
				prevUpdateStatusBlocks[i] = TaskHumanStatus(task.PrevUpdateStatusBlocks[i])
			}

			ep.prevUpdateStatusBlocks = prevUpdateStatusBlocks
		}

		errCreation := ep.CreateBlocks(ctx, version.Pipeline.Blocks)
		if errCreation != nil {
			log.Error(errCreation, ", work number: ", task.WorkNumber)
			outCh <- task.WorkNumber
			continue
		}

		variableStorage := store.NewStore()
		// TODO add finished
		// variableStorage.

		workNumber := task.WorkNumber

		steps, errSteps := p.db.GetTaskSteps(ctx, task.ID)
		if errSteps != nil {
			log.Error(errSteps, "work number: ", workNumber)
			outCh <- task.WorkNumber
			continue
		}

		sdState := &ApplicationData{}

		for j := range steps {
			if steps[j].Type != "servicedesk_application" {
				continue
			}
			step, errStep := p.db.GetTaskStepById(ctx, steps[j].ID)
			if errStep != nil {
				log.Error(errStep, "work number: ", workNumber)
				outCh <- task.WorkNumber
				break
			}

			// get sdState from step.State
			data, okState := step.State[steps[j].Name]
			if !okState {
				log.Error(fmt.Errorf(
					"can`t run pipeline with work number: %s, %s",
					workNumber,
					"sdState is`t found with name: "+steps[j].Name,
				))
				outCh <- task.WorkNumber
				break
			}

			if err := json.Unmarshal(data, sdState); err != nil {
				log.Error(err, "invalid format of servicedesk_application sdState, work number:", workNumber)
				outCh <- task.WorkNumber
				break
			}
		}

		if sdState.BlueprintID == "" {
			log.Error(fmt.Sprintf(
				"can`t run pipeline with work number: %s, %s",
				workNumber,
				"servicedesk_application block is not found",
			))

			outCh <- task.WorkNumber
			continue
		}

		ctx = c.WithValue(ctx, SdApplicationDataCtx{}, SdApplicationData{
			BlueprintID:     sdState.BlueprintID,
			Description:     sdState.Description,
			ApplicationBody: sdState.ApplicationBody,
		})

		ep.currDescription = sdState.Description

		go func(workNumber string) {
			routineCtx := c.WithValue(c.Background(), XRequestIDHeader, uuid.New().String())
			routineCtx = c.WithValue(routineCtx, SdApplicationDataCtx{}, ctx.Value(SdApplicationDataCtx{}))
			routineCtx = logger.WithLogger(routineCtx, log)
			err := ep.Run(routineCtx, variableStorage)
			if err != nil {
				isFailed = true
				log.Error(err, ", can`t run pipeline with number: ", workNumber)
				variableStorage.AddError(err)
			}
		}(workNumber)

		if isFailed {
			outCh <- workNumber
		}
	}
}

func (p *initiation) poolWorkers(
	ctx c.Context,
	wg *sync.WaitGroup,
	workers int,
	pipelines []entity.EriusTask,
	startedPipelinesCh chan string,
) {
	pipelinesCh := make(chan entity.EriusTask)

	for i := 0; i < workers; i++ {
		go p.worker(ctx, wg, pipelinesCh, startedPipelinesCh)
	}

	for i := range pipelines {
		pipelinesCh <- pipelines[i]
	}

	close(pipelinesCh)
}
