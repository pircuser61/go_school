package pipeline

import (
	c "context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
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
	clientIDKey  = "client_id"
	grantTypeKey = "grant_type"
	secretKey    = "client_secret"
	scopeKey     = "scope"

	// header values for tokens management
	grantTypeGetValue = "client_credentials"
)

func (runCtx BlockRunContext) addAuthHeader(ctx c.Context, r *http.Request) (err error) {
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
		token, tokenErr := runCtx.Services.Integrations.GetToken(ctx, oauthSecret.Scopes,
			oauthSecret.ClientSecret, oauthSecret.ClientId, oauthSecret.SSOStand)
		if tokenErr != nil {
			return tokenErr
		}
		r.Header.Add("Authorization", "Bearer "+token)
	case microservice_v1.AuthType_basicAuth.String():
		basicSecret := microservice_v1.BasicAuth{}
		err := json.Unmarshal(jsonbody, &basicSecret)
		if err != nil {
			return err
		}
		r.SetBasicAuth(basicSecret.Login, basicSecret.Pass)
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
