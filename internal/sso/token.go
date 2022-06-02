package sso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"
)

type SSOTokens struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

func (s *Service) registerScope(scopeName string) {
	scopes := []string{scopeName}
	if scopeName == "" {
		scopes = []string{}
	}
	if _, ok := s.scopes[scopeName]; ok {
		return
	}
	s.scopes[scopeName] = &scope{
		getTokensFormData: url.Values{
			secretKey:    []string{s.clientSecret},
			grantTypeKey: []string{grantTypeGetValue},
			clientIDKey:  []string{s.clientId},
			scopeKey:     scopes,
		},
	}
}

func getTokenInCookie(req *http.Request, name string) (string, error) {
	c, err := req.Cookie(name)
	if err != nil {
		return "", errors.New("can't find user session")
	}
	return c.Value, nil
}

func getTokenInBearer(req *http.Request) (string, error) {
	token := req.Header.Get(authHeader)
	if token == "" {
		return "", errors.New("can't find user session")
	}

	items := strings.Split(token, " ")
	if len(items) != 2 {
		return "", errors.New("got bad user session")
	}

	if items[0] != authType {
		return "", errors.New("can't find user session")
	}
	return items[1], nil
}

func (s *Service) GetAccessToken(req *http.Request) (string, error) {
	token, err := getTokenInBearer(req)
	if err != nil {
		token, err = getTokenInCookie(req, s.accessTokenCookieName)
		if err != nil {
			return "", err
		}
		return token, nil
	}
	return token, nil
}

func (s *Service) updateTokens(ctx context.Context, scopeName string) error {
	ctxLocal, span := trace.StartSpan(ctx, "updateTokens")
	defer span.End()

	currTime := time.Now()

	if _, ok := s.scopes[scopeName]; !ok {
		s.registerScope(scopeName)
	}
	sc := s.scopes[scopeName]

	if currTime.Before(sc.atExp) {
		return nil
	}

	if currTime.Before(sc.rtExp) {
		return s.refreshTokens(ctxLocal, scopeName)
	}

	return s.getTokens(ctxLocal, scopeName)
}

func (s *Service) getTokens(ctx context.Context, scopeName string) error {
	ctxLocal, span := trace.StartSpan(ctx, "getTokens")
	defer span.End()

	if _, ok := s.scopes[scopeName]; !ok {
		s.registerScope(scopeName)
	}
	sc := s.scopes[scopeName]

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodPost, s.tokensUrl, strings.NewReader(sc.getTokensFormData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add(contentTypeHeader, contentTypeFormValue)

	resp, err := s.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("got bad status code")
	}
	var res SSOTokens
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return unmErr
	}
	sc.accessToken = res.AccessToken
	sc.refreshToken = res.RefreshToken
	sc.atExp = time.Now().Add(time.Duration(res.ExpiresIn-expirationThreshold) * time.Second)
	sc.rtExp = time.Now().Add(time.Duration(res.RefreshExpiresIn-expirationThreshold) * time.Second)
	s.scopes[scopeName] = sc
	return nil
}

func (s *Service) refreshTokens(ctx context.Context, scopeName string) error {
	ctxLocal, span := trace.StartSpan(ctx, "refreshTokens")
	defer span.End()

	if _, ok := s.scopes[scopeName]; !ok {
		s.registerScope(scopeName)
	}
	sc := s.scopes[scopeName]

	formData := s.refreshTokensFormData
	formData.Add(refreshTokenKey, sc.refreshToken)

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodPost, s.tokensUrl, strings.NewReader(sc.getTokensFormData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add(contentTypeHeader, contentTypeFormValue)

	resp, err := s.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("got bad status code")
	}
	var res SSOTokens
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return unmErr
	}
	sc.accessToken = res.AccessToken
	sc.refreshToken = res.RefreshToken
	sc.atExp = time.Now().Add(time.Duration(res.ExpiresIn-expirationThreshold) * time.Second)
	sc.rtExp = time.Now().Add(time.Duration(res.RefreshExpiresIn-expirationThreshold) * time.Second)
	s.scopes[scopeName] = sc
	return nil
}
