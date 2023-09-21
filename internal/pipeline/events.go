package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	eventStart = "start"
	eventEnd   = "end"
)

func (runCtx *BlockRunContext) MakeNodeStartEvent(ctx c.Context, node string, humanStatus TaskHumanStatus,
	nodeStatus Status) (entity.NodeEvent, error) {
	if humanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return entity.NodeEvent{}, nil
		}
		humanStatus = TaskHumanStatus(hStatus)
	}

	return entity.NodeEvent{
		TaskID:     runCtx.TaskID.String(),
		WorkNumber: runCtx.WorkNumber,
		NodeName:   node,
		NodeStart:  time.Now().Format(time.RFC3339),
		TaskStatus: string(humanStatus),
		NodeStatus: string(nodeStatus),
	}, nil
}

func (runCtx *BlockRunContext) MakeNodeEndEvent(ctx c.Context, node string, humanStatus TaskHumanStatus,
	nodeStatus Status) (entity.NodeEvent, error) {
	if humanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return entity.NodeEvent{}, nil
		}
		humanStatus = TaskHumanStatus(hStatus)
	}

	outputs := getBlockOutput(runCtx.VarStore, node)

	return entity.NodeEvent{
		TaskID:     runCtx.TaskID.String(),
		WorkNumber: runCtx.WorkNumber,
		NodeName:   node,
		NodeStart:  runCtx.CurrBlockStartTime.Format(time.RFC3339),
		NodeEnd:    time.Now().Format(time.RFC3339),
		TaskStatus: string(humanStatus),
		NodeStatus: string(nodeStatus),
		NodeOutput: outputs,
	}, nil
}

func (runCtx BlockRunContext) NotifyEvents(ctx c.Context) {
	log := logger.GetLogger(ctx)

	reqUrl, err := url.Parse(runCtx.TaskSubscriptionData.MicroserviceURL)
	if err != nil {
		log.WithError(err).Error("couldn't parse url to send event notification")
		return
	}
	reqUrl.Path = path.Join(reqUrl.Path, runCtx.TaskSubscriptionData.NotificationPath)

	for i := range runCtx.BlockRunResults.NodeEvents {
		event := runCtx.BlockRunResults.NodeEvents[i]
		data, mapErr := script.MapData(runCtx.TaskSubscriptionData.Mapping, event.ToMap(), []string{})
		if mapErr != nil {
			log.WithError(mapErr).Error("couldn't map data")
			continue
		}
		body, jsonErr := json.Marshal(data)
		if jsonErr != nil {
			log.WithError(jsonErr).Error("couldn't marshal data")
			continue
		}
		req, reqErr := http.NewRequestWithContext(ctx, runCtx.TaskSubscriptionData.Method, reqUrl.String(),
			bytes.NewBuffer(body))
		if reqErr != nil {
			log.WithError(reqErr).Error("couldn't create request")
			continue
		}
		resp, respErr := runCtx.Services.HTTPClient.Do(req)
		if respErr != nil {
			log.WithError(respErr).Error("couldn't make request")
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.WithError(respErr).Error("didn't get 200 for request")
		}
	}
	return
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

func (runCtx *BlockRunContext) FillTaskEvents(ctx c.Context) (err error) {
	defer func() {
		if err != nil || runCtx.TaskSubscriptionData.ExpectedEvents == nil {
			log := logger.GetLogger(ctx)
			runCtx.TaskSubscriptionData.ExpectedEvents = make([]entity.NodeSubscriptionEvents, 0)
			log.Warning("FillTaskEvents got empty ExpectedEvents")
		}
	}()

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

	resp, err := runCtx.Services.Integrations.RpcMicrCli.GetMicroservice(ctx,
		&microservice_v1.GetMicroserviceRequest{MicroserviceId: expectedEvents.MicroserviceID})
	if err != nil {
		return err
	}
	if resp == nil || resp.Microservice == nil || resp.Microservice.Creds == nil || resp.Microservice.Creds.Prod == nil {
		return nil
	}

	runCtx.TaskSubscriptionData = TaskSubscriptionData{
		TaskRunClientID:    taskRunCtx.ClientID,
		SystemID:           sResp.Integration.IntegrationId,
		MicroserviceID:     expectedEvents.MicroserviceID,
		MicroserviceURL:    resp.Microservice.Creds.Prod.Addr,
		NotificationPath:   expectedEvents.Path,
		Method:             expectedEvents.Method,
		Mapping:            expectedEvents.Mapping,
		NotificationSchema: expectedEvents.NotificationSchema,
		ExpectedEvents:     expectedEvents.Nodes,
	}
	return nil
}
