package nocache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const (
	limitParam  = "limit"
	filterParam = "filter"
	sortByParam = "sortBy"

	usernamePH = "--!username!--"
	name1PH    = "--!name-1!--"
	name2PH    = "--!name-2!--"
	name3PH    = "--!name-3!--"

	sortByVal           = "lastName,firstName"
	companyFilterOption = `(attributes.OrgUnit co "%s")`
)

func (s *service) getUser(ctx context.Context, search string, onlyEnabled bool) ([]people.SSOUser, error) {
	search = strings.TrimSpace(search)
	if search == "" {
		return make([]people.SSOUser, 0), nil
	}

	ctxLocal, span := trace.StartSpan(ctx, "getUser")
	defer span.End()

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, s.searchURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()

	f := strings.Replace(usernameEqFilter, usernamePH, search, 1)
	if onlyEnabled {
		f = strings.Replace(usernameEqOnlyEnabledFilter, usernamePH, search, 1)
	}

	query.Add(filterParam, f)
	query.Add(limitParam, "1")
	query.Add(sortByParam, sortByVal)

	req.URL.RawQuery = query.Encode()

	resp, err := s.cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d for login: %s", resp.StatusCode, search)
	}

	var res people.SearchUsersResp

	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res.Resources, nil
}

func (s *service) getUsers(ctx context.Context, search string, limit int, filter []string) ([]people.SSOUser, error) {
	search = strings.TrimSpace(search)
	if search == "" {
		return make([]people.SSOUser, 0), nil
	}

	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.getUsers")
	defer span.End()

	var (
		req *retryablehttp.Request
		err error
	)

	req, err = retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, s.searchURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	const maxLimit = 100

	query := req.URL.Query()

	if limit > maxLimit {
		limit = maxLimit
	}

	f := defineFilter(search, false, filter)

	query.Add(filterParam, f)
	query.Add(limitParam, strconv.Itoa(limit))
	query.Add(sortByParam, sortByVal)

	req.URL.RawQuery = query.Encode()

	resp, err := s.cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d for login: %s", resp.StatusCode, search)
	}

	var res people.SearchUsersResp
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res.Resources, nil
}

func (s *service) PathBuilder(mainpath, subpath string) (string, error) {
	mu, err := url.Parse(mainpath)
	if err != nil {
		return "", err
	}

	mu.Path = path.Join(mu.Path, subpath)

	return mu.String(), nil
}

func (s *service) GetUserEmail(ctx context.Context, username string) (string, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_user_email")
	defer span.End()

	user, err := s.GetUser(ctxLocal, username)
	if err != nil {
		return "", err
	}

	typed, err := user.ToSSOUserTyped()
	if err != nil {
		return "", errors.Wrap(err, "couldn't convert user")
	}

	return typed.Email, nil
}

func (s *service) GetUser(ctx context.Context, username string) (people.SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_user")
	defer span.End()

	if sso.IsServiceUserName(username) {
		return map[string]interface{}{"username": username}, nil
	}

	users, err := s.getUser(ctxLocal, username, false)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		uname, ok := u["username"]
		if !ok {
			return nil, errors.New("couldn't find user with name " + username)
		}

		if uname == username {
			return u, nil
		}
	}

	return nil, errors.New("couldn't find user with name " + username)
}

func (s *service) GetUsers(ctx context.Context, username string, limit *int, filter []string) ([]people.SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_users")
	defer span.End()

	maxLimit := 0
	if limit != nil {
		maxLimit = *limit
	}

	users, err := s.getUsers(ctxLocal, username, maxLimit, filter)
	if err != nil {
		return nil, err
	}

	return users, nil
}
