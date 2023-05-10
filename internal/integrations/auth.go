package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.opencensus.io/trace"

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
type Auth struct {
	AuthType string `json:"auth"`
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
	Path     string `json:"path"`
	Token    string `json:"token,omitempty"`
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

	//nolint
	tokensPath = "/auth/realms/mts/protocol/openid-connect/token"

	mainSsoUrl = "https://isso%s.mts.ru"
)

func (s *Service) getToken(ctx context.Context, scopes []string, clientSecret, clientId, stand string) (token string, err error) {
	ctxLocal, span := trace.StartSpan(ctx, "getToken")
	defer span.End()

	initedScopes := s.initScopes(scopes, clientSecret, clientId)
	path := mainSsoUrl + tokensPath
	switch stand {
	case "dev", "stage":
		path = fmt.Sprintf(path, "-dev")
	case "prod":
		path = fmt.Sprintf(path, "")
	default:
		return "", errors.New("wrong stand name")
	}
	req, err := http.NewRequestWithContext(ctxLocal, http.MethodPost, path, strings.NewReader(initedScopes.getTokensFormData.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add(contentTypeHeader, contentTypeFormValue)

	resp, err := s.Cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("got bad status code")
	}
	var res SSOToken
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return "", unmErr
	}

	return res.AccessToken, nil
}

func (s *Service) initScopes(scopes []string, clientSecret, clientId string) *scope {
	return &scope{
		getTokensFormData: url.Values{
			secretKey:    []string{clientSecret},
			grantTypeKey: []string{grantTypeGetValue},
			clientIDKey:  []string{clientId},
			scopeKey:     []string{strings.Join(scopes, " ")},
		},
	}
}

func (s *Service) FillAuth(ctx context.Context, key string) (result *Auth, err error) {
	res, GRPCerr := s.RpcMicrCli.GetCredentialsByKey(ctx,
		&microservice_v1.GetCredentialsByKeyRequest{HumanReadableKey: key},
	)
	if GRPCerr != nil {
		return nil, GRPCerr
	}
	if res.Auth.Type == microservice_v1.AuthType_basicAuth {
		result = &Auth{
			AuthType: "basicAuth",
			Login:    res.Auth.GetBasic().Login,
			Password: res.Auth.GetBasic().Pass,
			Path:     res.Auth.Addr,
		}
	} else {
		oauthGrpc := res.Auth.GetOAuth2()
		token, tokenErr := s.getToken(ctx, oauthGrpc.Scopes, oauthGrpc.ClientSecret, oauthGrpc.ClientId, oauthGrpc.SSOStand)
		if tokenErr != nil {
			return nil, tokenErr
		}
		result = &Auth{
			AuthType: "oAuth",
			Token:    token,
			Path:     res.Auth.Addr,
		}
	}

	return result, nil
}
