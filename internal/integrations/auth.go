package integrations

import (
	c "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.opencensus.io/trace"

	rhttp "github.com/hashicorp/go-retryablehttp"

	microservice "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
)

type SSOToken struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

type Auth struct {
	AuthType string `json:"auth"`
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
	Path     string `json:"path"`
	Token    string `json:"token,omitempty"`
}

type scope struct {
	getTokensFormData url.Values
}

const (
	clientIDKey  = "client_id"
	grantTypeKey = "grant_type"
	secretKey    = "client_secret"
	scopeKey     = "scope"

	// header keys for tokens management
	contentTypeHeader = "Content-Type"

	// header values for tokens management
	contentTypeFormValue = "application/x-www-form-urlencoded"
	grantTypeGetValue    = "client_credentials"

	// //nolint:gosec // just a path
	tokensPath = "/auth/realms/mts/protocol/openid-connect/token"

	mainSsoURL = "https://isso%s.mts.ru"
)

func (s *service) GetToken(ctx c.Context, scopes []string, clientSecret, clientID, stand string) (token string, err error) {
	ctxLocal, span := trace.StartSpan(ctx, "getToken")
	defer span.End()

	sc := s.initScopes(scopes, clientSecret, clientID)
	path := mainSsoURL + tokensPath

	switch stand {
	case "dev", "stage":
		path = fmt.Sprintf(path, "-dev")
	case "prod":
		path = fmt.Sprintf(path, "")
	default:
		return "", errors.New("wrong stand name")
	}

	req, err := rhttp.NewRequestWithContext(ctxLocal, http.MethodPost, path, strings.NewReader(sc.getTokensFormData.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add(contentTypeHeader, contentTypeFormValue)

	resp, err := s.cli.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got bad status code %d", resp.StatusCode)
	}

	var res SSOToken

	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return "", unmErr
	}

	return res.AccessToken, nil
}

func (s *service) initScopes(scopes []string, clientSecret, clientID string) *scope {
	return &scope{
		getTokensFormData: url.Values{
			secretKey:    []string{clientSecret},
			grantTypeKey: []string{grantTypeGetValue},
			clientIDKey:  []string{clientID},
			scopeKey:     []string{strings.Join(scopes, " ")},
		},
	}
}

func (s *service) FillAuth(ctx c.Context, key, pID, vID, wNumber, clientID string) (res *Auth, err error) {
	cred, grpcErr := s.rpcMicrCli.GetCredentialsByKey(ctx,
		&microservice.GetCredentialsByKeyRequest{
			HumanReadableKey: key,
			PipelineId:       pID,
			VersionId:        vID,
			WorkNumber:       wNumber,
			ClientId:         clientID,
		},
	)
	if grpcErr != nil {
		return nil, grpcErr
	}

	switch cred.Auth.Type {
	case microservice.AuthType_basicAuth:
		res = &Auth{
			AuthType: "basicAuth",
			Login:    cred.Auth.GetBasic().Login,
			Password: cred.Auth.GetBasic().Pass,
			Path:     cred.Auth.Addr,
		}
	case microservice.AuthType_oAuth2:
		oauthGrpc := cred.Auth.GetOAuth2()

		token, tokenErr := s.GetToken(ctx, oauthGrpc.Scopes, oauthGrpc.ClientSecret, oauthGrpc.ClientId, oauthGrpc.SSOStand)
		if tokenErr != nil {
			return nil, tokenErr
		}

		res = &Auth{
			AuthType: "oAuth",
			Token:    token,
			Path:     cred.Auth.Addr,
		}
	case microservice.AuthType_bearerToken:
		res = &Auth{
			AuthType: "bearerToken",
			Token:    cred.Auth.GetBearerToken().Token,
			Path:     cred.Auth.Addr,
		}
	}

	return res, nil
}
