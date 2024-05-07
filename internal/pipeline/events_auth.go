package pipeline

import (
	c "context"
	"encoding/json"

	"github.com/hashicorp/go-retryablehttp"

	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
)

type SSOToken struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

//nolint:gocritic //поинтер изначально не был предусмотрен
func (runCtx BlockRunContext) addAuthHeader(ctx c.Context, r *retryablehttp.Request) error {
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

		token, tokenErr := runCtx.Services.Integrations.GetToken(
			ctx,
			oauthSecret.Scopes,
			oauthSecret.ClientSecret,
			oauthSecret.ClientId,
			oauthSecret.SSOStand,
		)
		if tokenErr != nil {
			return tokenErr
		}

		//nolint:goconst //не хочу внедрять миллион констант под каждую строку в проекте
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
	}

	return nil
}
