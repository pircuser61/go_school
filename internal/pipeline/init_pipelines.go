package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
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
	db db.Database,
	faaS string,
	httpClient *http.Client,
	remedy string,
	sender *mail.Service,
	serviceDesc *servicedesc.Service,
	peopleSrv *people.Service,
) *initiation {
	return &initiation{
		db:          db,
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

	unfinished, err := p.db.GetUnfinishedTasks(ctx)
	if err != nil {
		return err
	}

	if unfinished == nil {
		return errors.New("can`t init pipelines, empty tasks")
	}

	log.Info(fmt.Sprintf("--- init pipelines count: %d ---", len(unfinished.Tasks)))

	for i := range unfinished.Tasks {
		version, errVersion := p.db.GetVersionByWorkNumber(ctx, unfinished.Tasks[i].WorkNumber)
		if errVersion != nil {
			return errVersion
		}

		ep := ExecutablePipeline{}
		ep.PipelineID = unfinished.Tasks[i].ID
		ep.VersionID = unfinished.Tasks[i].VersionID
		ep.Storage = p.db
		ep.EntryPoint = version.Pipeline.Entrypoint
		ep.FaaS = p.faaS
		ep.PipelineModel = version
		ep.HTTPClient = p.httpClient
		ep.Remedy = p.remedy
		ep.ActiveBlocks = map[string]struct{}{}
		ep.SkippedBlocks = map[string]struct{}{}
		ep.EntryPoint = BlockGoFirstStart
		ep.Sender = p.sender
		ep.People = p.peopleSrv
		ep.Name = unfinished.Tasks[i].Name
		ep.Initiator = unfinished.Tasks[i].Author
		ep.ServiceDesc = p.serviceDesc

		variableStorage := store.NewStore()

		workNumber := unfinished.Tasks[i].WorkNumber

		steps, errSteps := p.db.GetTaskSteps(ctx, unfinished.Tasks[i].ID)
		if errSteps != nil {
			return errSteps
		}

		state := &ApplicationData{}

		for j := range steps {
			if steps[j].Type == "servicedesk_application" {
				step, errStep := p.db.GetTaskStepById(ctx, steps[j].ID)
				if errStep != nil {
					return errStep
				}

				// get state from step.State
				data, ok := step.State[steps[j].Name]
				if !ok {
					return fmt.Errorf(
						"can`t run pipeline with work number: %s, %s",
						unfinished.Tasks[i].WorkNumber,
						"state is`t found with name: "+steps[j].Name,
					)
				}

				err = json.Unmarshal(data, state)
				if err != nil {
					return errors.Wrap(err, "invalid format of servicedesk_application state")
				}

				break
			}

			return fmt.Errorf(
				"can`t run pipeline with work number: %s, %s",
				unfinished.Tasks[i].WorkNumber,
				"servicedesk_application block is not found",
			)
		}

		ctx = c.WithValue(ctx, SdApplicationDataCtx{}, SdApplicationData{
			BlueprintID:     state.BlueprintID,
			Description:     state.Description,
			ApplicationBody: state.ApplicationBody,
		})

		go func(workNumber string) {
			routineCtx := c.WithValue(c.Background(), XRequestIDHeader, uuid.New().String())
			routineCtx = c.WithValue(routineCtx, SdApplicationDataCtx{}, ctx.Value(SdApplicationDataCtx{}))
			routineCtx = logger.WithLogger(routineCtx, log)
			err = ep.Run(routineCtx, variableStorage)
			if err != nil {
				log.Error("can`t run pipeline with number: ", workNumber)
				variableStorage.AddError(err)
			}
		}(workNumber)
	}

	return nil
}
