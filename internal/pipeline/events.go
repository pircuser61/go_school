package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/fatih/structs"

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

type MakeNodeStartEventArgs struct {
	NodeName      string
	NodeShortName string
	HumanStatus   TaskHumanStatus
	NodeStatus    Status
}

type MakeNodeEndEventArgs struct {
	NodeName      string
	NodeShortName string
	HumanStatus   TaskHumanStatus
	NodeStatus    Status
}

func (runCtx *BlockRunContext) MakeNodeStartEvent(ctx c.Context, args MakeNodeStartEventArgs) (entity.NodeEvent, error) {
	if args.HumanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return entity.NodeEvent{}, nil
		}
		args.HumanStatus = TaskHumanStatus(hStatus)
	}

	return entity.NodeEvent{
		TaskID:        runCtx.TaskID.String(),
		WorkNumber:    runCtx.WorkNumber,
		NodeName:      args.NodeName,
		NodeShortName: args.NodeShortName,
		NodeStart:     time.Now().Format(time.RFC3339),
		TaskStatus:    string(args.HumanStatus),
		NodeStatus:    string(args.NodeStatus),
	}, nil
}

func (runCtx *BlockRunContext) MakeNodeEndEvent(ctx c.Context, args MakeNodeEndEventArgs) (entity.NodeEvent, error) {
	if args.HumanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return entity.NodeEvent{}, nil
		}
		args.HumanStatus = TaskHumanStatus(hStatus)
	}

	outputs := getBlockOutput(runCtx.VarStore, args.NodeName)

	return entity.NodeEvent{
		TaskID:        runCtx.TaskID.String(),
		WorkNumber:    runCtx.WorkNumber,
		NodeName:      args.NodeName,
		NodeShortName: args.NodeShortName,
		NodeStart:     runCtx.CurrBlockStartTime.Format(time.RFC3339),
		NodeEnd:       time.Now().Format(time.RFC3339),
		TaskStatus:    string(args.HumanStatus),
		NodeStatus:    string(args.NodeStatus),
		NodeOutput:    outputs,
	}, nil
}

func (runCtx BlockRunContext) NotifyEvents(ctx c.Context) {
	log := logger.GetLogger(ctx).WithField("workNumber", runCtx.WorkNumber)

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
		headerErr := runCtx.addAuthHeader(ctx, req)
		if headerErr != nil {
			log.WithError(reqErr).Error("couldn't add auth Headers")
			continue
		}
		log.Info(req.Header.Get("Authorization"))
		resp, respErr := runCtx.Services.HTTPClient.Do(req)
		if respErr != nil {
			log.WithError(respErr).Error("couldn't make request")
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errMsg := fmt.Sprintf("didn't get 200 for request, got %d", resp.StatusCode)
			log.Error(errMsg)
		}
	}
	return
}

func (runCtx BlockRunContext) GetCancelledStepsEvents(ctx c.Context) ([]entity.NodeEvent, error) {
	steps, err := runCtx.Services.Storage.GetCanceledTaskSteps(ctx, runCtx.TaskID)
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

		nodeEvent := MakeNodeEndEventArgs{
			NodeName:    s.Name,
			HumanStatus: StatusRevoke,
			NodeStatus:  StatusCanceled,
		}

		if s.ShortTitle != nil {
			nodeEvent.NodeShortName = *s.ShortTitle
		}

		event, eventErr := runCtx.MakeNodeEndEvent(ctx, nodeEvent)
		if eventErr != nil {
			return nil, eventErr
		}
		nodeEvents = append(nodeEvents, event)
	}

	return nodeEvents, nil
}

func (runCtx *BlockRunContext) SetTaskEvents(ctx c.Context) {
	var err error
	defer func() {
		if err != nil {
			log := logger.GetLogger(ctx).WithField("funcName", "setTaskEvents")
			log.WithField("workNumber", runCtx.WorkNumber).Warning(err)
		}

		if runCtx.TaskSubscriptionData.ExpectedEvents == nil {
			runCtx.TaskSubscriptionData.ExpectedEvents = make([]entity.NodeSubscriptionEvents, 0)
		}
	}()

	taskRunCtx, err := runCtx.Services.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if err != nil {
		return
	}

	sResp, err := runCtx.Services.Integrations.RpcIntCli.GetIntegrationByClientId(ctx,
		&integration_v1.GetIntegrationByClientIdRequest{ClientId: taskRunCtx.ClientID})
	if err != nil {
		return
	}
	if sResp == nil || sResp.Integration == nil {
		return
	}

	expectedEvents, err := runCtx.Services.Storage.GetTaskEventsParamsByWorkNumber(ctx,
		runCtx.WorkNumber, sResp.Integration.IntegrationId)
	if err != nil {
		return
	}
	if expectedEvents.SystemID == "" {
		return
	}

	resp, err := runCtx.Services.Integrations.RpcMicrCli.GetMicroservice(ctx,
		&microservice_v1.GetMicroserviceRequest{MicroserviceId: expectedEvents.MicroserviceID})
	if err != nil {
		return
	}
	if resp == nil || resp.Microservice == nil || resp.Microservice.Creds == nil || resp.Microservice.Creds.Prod == nil {
		return
	}

	runCtx.TaskSubscriptionData = TaskSubscriptionData{
		TaskRunClientID:      taskRunCtx.ClientID,
		SystemID:             sResp.Integration.IntegrationId,
		MicroserviceID:       expectedEvents.MicroserviceID,
		MicroserviceURL:      resp.Microservice.Creds.Prod.Addr,
		MicroserviceAuthType: resp.Microservice.Creds.Prod.Type.String(),
		MicroserviceSecrets:  fillSecrets(resp.Microservice.Creds.Prod),
		NotificationPath:     expectedEvents.Path,
		Method:               expectedEvents.Method,
		Mapping:              expectedEvents.Mapping,
		NotificationSchema:   expectedEvents.NotificationSchema,
		ExpectedEvents:       expectedEvents.Nodes,
	}

	return
}

func fillSecrets(a *microservice_v1.Auth) (result map[string]interface{}) {
	switch a.Type {
	case microservice_v1.AuthType_oAuth2:
		return structs.Map(a.GetOAuth2())
	case microservice_v1.AuthType_basicAuth:
		return structs.Map(a.GetBasic())
	case microservice_v1.AuthType_bearerToken:
		return structs.Map(a.GetBearerToken())
	default:
		return nil
	}
}
