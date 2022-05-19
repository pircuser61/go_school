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

	"gopkg.in/square/go-jose.v2/jwt"
)

const (
	userinfoPath = "auth/realms/%s/protocol/openid-connect/userinfo"

	// headers keys common
	authHeader = "Authorization"

	authType = "Bearer"

	// headers values common
	authBearerValue = "Bearer %s"

	cacheTTL = time.Minute * 10
)

type cachedUserInfo struct {
	u    *UserInfo
	till time.Time
}

type Service struct {
	mainUrl     string
	userinfoUrl string

	clientSecret     string
	clientId         string
	clientIdentifier string

	accessTokenCookieName string

	realm         string
	userInfoCache map[string]*cachedUserInfo
	userInfoMutex *sync.RWMutex

	cli *http.Client
}

// nolint:gocritic // it's more comfortable to work with config as a value
func NewService(c Config, cli *http.Client) (*Service, error) {
	s := &Service{
		mainUrl:               c.Address,
		realm:                 c.Realm,
		clientSecret:          os.Getenv(c.ClientSecretEnvKey),
		clientId:              c.ClientID,
		accessTokenCookieName: c.AccessTokenCookieName,
		cli:                   cli,
		userInfoCache:         map[string]*cachedUserInfo{},
		userInfoMutex:         &sync.RWMutex{},
	}

	if err := s.buildAllPaths(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Service) buildAllPaths() error {
	uip, err := s.pathBuilder(s.mainUrl, userinfoPath, false)
	if err != nil {
		return err
	}

	s.userinfoUrl = uip

	return nil
}

func (s *Service) pathBuilder(mainpath, subpath string, addClient bool) (string, error) {
	mu, err := url.Parse(mainpath)
	if err != nil {
		return "", err
	}
	if addClient {
		subpath = fmt.Sprintf(subpath, s.realm, s.clientIdentifier)
	} else {
		subpath = fmt.Sprintf(subpath, s.realm)
	}
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
