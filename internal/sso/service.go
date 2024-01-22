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
	mainUrl     string
	tokensUrl   string
	userinfoUrl string

	clientSecret string
	clientId     string

	accessTokenCookieName string

	realm                 string
	scopes                map[string]*scope
	scopesMutex           *sync.RWMutex
	refreshTokensFormData url.Values
	userInfoCache         map[string]*cachedUserInfo
	userInfoMutex         *sync.RWMutex

	cli *http.Client
}

// nolint:gocritic // it's more comfortable to work with config as a value
func NewService(c Config, cli *http.Client) (*Service, error) {
	refreshFD := url.Values{
		grantTypeKey: []string{grantTypeRefreshValue},
		clientIDKey:  []string{c.ClientID},
	}

	s := &Service{
		scopes:                make(map[string]*scope),
		mainUrl:               c.Address,
		realm:                 c.Realm,
		clientSecret:          os.Getenv(c.ClientSecretEnvKey),
		clientId:              c.ClientID,
		accessTokenCookieName: c.AccessTokenCookieName,
		cli:                   cli,
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
	tp, err := s.pathBuilder(s.mainUrl, tokensPath)
	if err != nil {
		return err
	}

	uip, err := s.pathBuilder(s.mainUrl, userinfoPath)
	if err != nil {
		return err
	}

	s.tokensUrl = tp
	s.userinfoUrl = uip

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
