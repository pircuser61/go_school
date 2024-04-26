package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"
)

type UserInfoResourceAccess struct {
	JocastaDEV   UserInfoResourceAccessItem `json:"jocasta-dev"`
	JocastaStage UserInfoResourceAccessItem `json:"jocasta-stage"`
	JocastaProd  UserInfoResourceAccessItem `json:"jocasta-prod"`
}

type UserInfoResourceAccessItem struct {
	Roles []string `json:"roles"`
}

type UserInfo struct {
	ResourceAccess    UserInfoResourceAccess `json:"resource_access"`
	Aud               []string               `json:"aud"`
	AZP               string                 `json:"azp"`
	Email             string                 `json:"email"`
	EmployeeID        string                 `json:"employeeID"`
	FamilyName        string                 `json:"family_name"`
	FullName          string                 `json:"fullname"`
	GivenName         string                 `json:"given_name"`
	Name              string                 `json:"name"`
	PhoneNumber       string                 `json:"phone_number"`
	PreferredUsername string                 `json:"preferred_username"`
	Title             string                 `json:"title"`
	Username          string                 `json:"username"`
	ThumbnailPhoto    string                 `json:"thumbnailPhoto"`
	Company           string                 `json:"company"`
	MemberOf          []string               `json:"memberOf"`
	OrgUnit           string                 `json:"OrgUnit"`
	ProxyEmails       []string               `json:"proxyAddresses"`
}

const (
	autoApproval = "auto_approve"
	autoSigner   = "auto_signer"
	autoFill     = "auto_fill"
)

type custClaims struct {
	PrefName string `json:"preferred_username"`
	Username string `json:"username"`
}

func (s *Service) userinfoToCache(username string, userinfo *UserInfo) {
	s.userInfoMutex.Lock()
	s.userInfoCache[username] = &cachedUserInfo{
		u:    userinfo,
		till: time.Now().UTC().Add(cacheTTL),
	}
	s.userInfoMutex.Unlock()
}

func (s *Service) userinfoFromCache(username string) *UserInfo {
	s.userInfoMutex.RLock()
	res, ok := s.userInfoCache[username]
	s.userInfoMutex.RUnlock()

	if ok {
		if time.Now().UTC().Before(res.till) {
			return res.u
		}

		s.userInfoMutex.Lock()
		delete(s.userInfoCache, username)
		s.userInfoMutex.Unlock()
	}

	return nil
}

func (s *Service) getUserinfo(ctx context.Context, r *http.Request) (*UserInfo, error) {
	ctxLocal, span := trace.StartSpan(ctx, "GetUserinfo")
	defer span.End()

	var token string

	token, err := s.GetAccessToken(r)
	if err != nil {
		return nil, err
	}

	if token == "" {
		return nil, errors.New("can't find token")
	}

	username, err := getUsername(token)
	if err != nil {
		return nil, err
	}

	if userinfo := s.userinfoFromCache(username); userinfo != nil {
		return userinfo, nil
	}

	if IsServiceUserName(username) {
		return &UserInfo{
			Username: username,
		}, nil
	}

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, s.userinfoURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Add(authHeader, fmt.Sprintf(authBearerValue, token))

	resp, err := s.cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, errors.New("got no access token to make request")
	case http.StatusOK:
	default:
		return nil, errors.Errorf("got bad status code %d", resp.StatusCode)
	}

	var user *UserInfo
	if unmErr := json.NewDecoder(resp.Body).Decode(&user); unmErr != nil {
		return nil, unmErr
	}

	s.userinfoToCache(username, user)

	return user, nil
}

func IsServiceUserName(username string) bool {
	if strings.HasPrefix(username, "service-account") {
		return true
	}

	if username == autoApproval {
		return true
	}

	if username == autoSigner {
		return true
	}

	if username == autoFill {
		return true
	}

	return false
}
