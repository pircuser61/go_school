package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/fatih/structs"
	"go.opencensus.io/trace"
	"golang.org/x/net/context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type SSOToken struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

type scope struct {
	getTokensFormData url.Values
}

const (
	eventStart = "start"
	eventEnd   = "end"

	clientIDKey  = "client_id"
	grantTypeKey = "grant_type"
	secretKey    = "client_secret"
	scopeKey     = "scope"

	// header keys for tokens management
	contentTypeHeader = "Content-Type"

	// header values for tokens management
	contentTypeFormValue = "application/x-www-form-urlencoded"
	grantTypeGetValue    = "client_credentials"

	tokensPath = "https://isso.mts.ru/auth/realms/mts/protocol/openid-connect/token"
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
		headerErr := runCtx.addAuthHeader(ctx, req)
		if headerErr != nil {
			log.WithError(reqErr).Error("couldn't add auth Headers")
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

func (runCtx BlockRunContext) addAuthHeader(ctx context.Context, r *http.Request) (err error) {
	jsonbody, err := json.Marshal(runCtx.TaskSubscriptionData.MicroserviceSecrets)
	if err != nil {
		return err
	}
	switch runCtx.TaskSubscriptionData.MicroserviceAuthType {
	case microservice_v1.AuthType_oAuth2.String():
		oauthSecret := microservice_v1.OAuth2{}
		err := json.Unmarshal(jsonbody, &oauthSecret)
		if err != nil {
			return err
		}
		token, tokenErr := runCtx.getToken(ctx, &oauthSecret)
		if tokenErr != nil {
			return tokenErr
		}
		r.Header.Add("Authorization", token)
	case microservice_v1.AuthType_basicAuth.String():
		basicSecret := microservice_v1.BasicAuth{}
		err := json.Unmarshal(jsonbody, &basicSecret)
		if err != nil {
			return err
		}
		r.Header.Add("login", basicSecret.Login)
		r.Header.Add("password", basicSecret.Pass)
	case microservice_v1.AuthType_bearerToken.String():
		bearerSecret := microservice_v1.BearerToken{}
		err := json.Unmarshal(jsonbody, &bearerSecret)
		if err != nil {
			return err
		}
		r.Header.Add("Authorization", bearerSecret.Token)
	default:
		return
	}
	return nil
}
func (runCtx BlockRunContext) getToken(ctx context.Context, authdata *microservice_v1.OAuth2) (token string, err error) {
	ctxLocal, span := trace.StartSpan(ctx, "getToken")
	defer span.End()

	initedScopes := runCtx.initScopes(authdata.Scopes, authdata.ClientSecret, authdata.ClientId)

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodPost, tokensPath, strings.NewReader(initedScopes.getTokensFormData.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add(contentTypeHeader, contentTypeFormValue)

	resp, err := runCtx.Services.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code while geting Notif Token")
	}
	var res SSOToken
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return "", unmErr
	}

	return res.AccessToken, nil
}

func (runCtx BlockRunContext) initScopes(scopes []string, clientSecret, clientId string) *scope {
	return &scope{
		getTokensFormData: url.Values{
			secretKey:    []string{clientSecret},
			grantTypeKey: []string{grantTypeGetValue},
			clientIDKey:  []string{clientId},
			scopeKey:     []string{strings.Join(scopes, " ")},
		},
	}
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
		structs.Map(a.GetBasic())
	case microservice_v1.AuthType_bearerToken:
		structs.Map(a.GetBearerToken())
	default:
		return nil
	}
	return nil
}
