package sso

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	"go.opencensus.io/trace"

	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/hashicorp/go-retryablehttp"
	"gitlab.services.mts.ru/abp/myosotis/observability"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"go.opencensus.io/plugin/ochttp"
)

const (
	//nolint:gosec // just a path
	tokensPath   = "auth/realms/%s/protocol/openid-connect/token"
	userinfoPath = "auth/realms/%s/protocol/openid-connect/userinfo"

	// headers keys common
	authHeader = "Authorization"

	authType = "Bearer"

	// form data keys for token management
	clientIDKey     = "client_id"
	grantTypeKey    = "grant_type"
	refreshTokenKey = "refresh_token"
	secretKey       = "client_secret"
	scopeKey        = "scope"

	// form data values for token management
	grantTypeRefreshValue = "refresh_token"
	grantTypeGetValue     = "client_credentials"

	// headers values common
	authBearerValue = "Bearer %s"

	// header keys for tokens management
	contentTypeHeader = "Content-Type"

	// header values for tokens management
	contentTypeFormValue = "application/x-www-form-urlencoded"

	expirationThreshold = 15

	cacheTTL = time.Minute * 10
)

type cachedUserInfo struct {
	u    *UserInfo
	till time.Time
}

type scope struct {
	accessToken  string
	atExp        time.Time
	refreshToken string
	rtExp        time.Time

	getTokensFormData url.Values
}

type Service struct {
	mainURL     string
	tokensURL   string
	userinfoURL string

	clientSecret string
	clientID     string

	accessTokenCookieName string

	realm                 string
	scopes                map[string]*scope
	scopesMutex           *sync.RWMutex
	refreshTokensFormData url.Values
	userInfoCache         map[string]*cachedUserInfo
	userInfoMutex         *sync.RWMutex

	cli *retryablehttp.Client
}

// nolint:gocritic // it's more comfortable to work with config as a value
func NewService(c Config) (*Service, error) {
	httpClient := &http.Client{}

	tr := TransportForSso{
		Transport: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		Scope: "",
	}
	httpClient.Transport = &tr
	newCli := httpclient.NewClient(httpClient, nil, c.MaxRetries, c.RetryDelay)

	refreshFD := url.Values{
		grantTypeKey: []string{grantTypeRefreshValue},
		clientIDKey:  []string{c.ClientID},
	}

	s := &Service{
		scopes:                make(map[string]*scope),
		mainURL:               c.Address,
		realm:                 c.Realm,
		clientSecret:          os.Getenv(c.ClientSecretEnvKey),
		clientID:              c.ClientID,
		accessTokenCookieName: c.AccessTokenCookieName,
		cli:                   newCli,
		refreshTokensFormData: refreshFD,
		userInfoCache:         map[string]*cachedUserInfo{},
		userInfoMutex:         &sync.RWMutex{},
		scopesMutex:           &sync.RWMutex{},
	}

	if err := s.buildAllPaths(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Service) buildAllPaths() error {
	tp, err := s.pathBuilder(s.mainURL, tokensPath)
	if err != nil {
		return err
	}

	uip, err := s.pathBuilder(s.mainURL, userinfoPath)
	if err != nil {
		return err
	}

	s.tokensURL = tp
	s.userinfoURL = uip

	return nil
}

func (s *Service) pathBuilder(mainpath, subpath string) (string, error) {
	mu, err := url.Parse(mainpath)
	if err != nil {
		return "", err
	}

	subpath = fmt.Sprintf(subpath, s.realm)

	mu.Path = path.Join(mu.Path, subpath)

	return mu.String(), nil
}

func getUsername(token string) (string, error) {
	parsed, err := jwt.ParseSigned(token)
	if err != nil {
		return "", err
	}

	cl := custClaims{}
	if err := parsed.UnsafeClaimsWithoutVerification(&cl); err != nil {
		return "", err
	}

	if cl.Username == "" {
		return cl.PrefName, nil
	}

	return cl.Username, nil
}

func (s *Service) GetUserinfo(ctx context.Context, r *http.Request) (*UserInfo, error) {
	userinfo, err := s.getUserinfo(ctx, r)
	if err != nil {
		return nil, err
	}

	return userinfo, nil
}

func (s *Service) BindAuthHeader(ctx context.Context, req *http.Request, scopeName string) error {
	ctxLocal, span := trace.StartSpan(ctx, "GetUserinfo")
	defer span.End()

	if tokenErr := s.updateTokens(ctxLocal, scopeName); tokenErr != nil {
		return tokenErr
	}

	req.Header.Del(authHeader)

	s.scopesMutex.RLock()
	req.Header.Add(authHeader, fmt.Sprintf(authBearerValue, s.scopes[scopeName].accessToken))
	s.scopesMutex.RUnlock()

	return nil
}
