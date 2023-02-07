package people

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const (
	searchPath = "search/attributes"

	limitParam  = "limit"
	filterParam = "filter"
	sortByParam = "sortBy"

	usernamePH = "--!username!--"
	name1PH    = "--!name-1!--"
	name2PH    = "--!name-2!--"
	name3PH    = "--!name-3!--"

	sortByVal = "lastName,firstName"
)

var (
	englishCheck = regexp.MustCompile(`[^а-яё]`)
	newStringRm  = regexp.MustCompile(`(\s\s+)|(\n)`)

	usernameEqFilter = fmt.Sprintf("(username eq %q)", usernamePH)

	usernameFilter = fmt.Sprintf("(username sw %q)", usernamePH)
	onePartFilter  = fmt.Sprintf("(lastName sw %q)", name1PH)
	twoPartFilter  = fmt.Sprintf(`(((firstName eq "%s") and (lastName sw "%s")) or
											((firstName sw "%s") and (lastName eq "%s"))
)`,
		name1PH, name2PH,
		name2PH, name1PH)
	threePartFilter = fmt.Sprintf(`(((lastName eq "%s") and (firstName eq "%s") and (attributes.fullname co "%s")) or 
											((lastName eq "%s") and (firstName eq "%s") and (attributes.fullname co "%s")) or 
											((lastName eq "%s") and (firstName sw "%s") and (attributes.fullname co "%s")) or 
											((lastName eq "%s") and (firstName sw "%s") and (attributes.fullname co "%s")) or 
											((lastName sw "%s") and (firstName eq "%s") and (attributes.fullname co "%s")) or 
											((lastName sw "%s") and (firstName eq "%s") and (attributes.fullname co "%s"))  
)`,
		name1PH, name2PH, name3PH,
		name2PH, name1PH, name3PH,
		name1PH, name3PH, name2PH,
		name2PH, name3PH, name1PH,
		name3PH, name1PH, name2PH,
		name3PH, name2PH, name1PH,
	)
	companyFilterOption = `(attributes.OrgUnit co "%s")`
)

// nolint:staticcheck // Cant use cases.Title
func defineFilter(input string, oneWord bool, filter []string) string {
	parts := strings.Split(strings.ToLower(input), " ")
	var q string
	if len(parts) == 1 || oneWord {
		if englishCheck.FindString(strings.ToLower(parts[0])) != "" {
			q = strings.Replace(usernameFilter, usernamePH, parts[0], 1)
		} else {
			q = strings.Replace(onePartFilter, name1PH, strings.Title(parts[0]), 1)
		}
	}
	if len(parts) == 2 {
		q = strings.Replace(twoPartFilter, name1PH, strings.Title(parts[0]), -1)
		q = strings.Replace(q, name2PH, strings.Title(parts[1]), -1)
	}
	if len(parts) > 2 {
		q = strings.Replace(threePartFilter, name1PH, strings.Title(parts[0]), -1)
		q = strings.Replace(q, name2PH, strings.Title(parts[1]), -1)
		q = strings.Replace(q, name3PH, strings.Title(parts[2]), -1)
	}
	if len(filter) > 0 {
		var companyOptions []string
		for _, f := range filter {
			companyOptions = append(companyOptions, fmt.Sprintf(companyFilterOption, f))
		}
		q += fmt.Sprintf(" and (%s)", strings.Join(companyOptions, " or "))
	}
	return newStringRm.ReplaceAllString(q, " ")
}

type TransportForPeople struct {
	transport ochttp.Transport
	sso       *sso.Service
	scope     string
}

func (t *TransportForPeople) RoundTrip(req *http.Request) (*http.Response, error) {
	err := t.sso.BindAuthHeader(req.Context(), req, t.scope)
	if err != nil {
		return nil, err
	}

	return t.transport.RoundTrip(req)
}

type Service struct {
	searchUrl string

	cli *http.Client
	sso *sso.Service
}

func NewService(c Config, ssoS *sso.Service) (*Service, error) {
	newCli := &http.Client{}

	tr := TransportForPeople{
		transport: ochttp.Transport{
			Base:        newCli.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: "",
	}
	newCli.Transport = &tr

	s := &Service{
		cli: newCli,
		sso: ssoS,
	}

	search, err := s.pathBuilder(c.Url, searchPath)
	if err != nil {
		return nil, err
	}
	s.searchUrl = search

	return s, nil
}

func (s *Service) pathBuilder(mainpath, subpath string) (string, error) {
	mu, err := url.Parse(mainpath)
	if err != nil {
		return "", err
	}
	mu.Path = path.Join(mu.Path, subpath)
	return mu.String(), nil
}

func (s *Service) GetUserEmail(ctx context.Context, username string) (string, error) {
	ctxLocal, span := trace.StartSpan(ctx, "GetUserEmail")
	defer span.End()

	users, err := s.getUser(ctxLocal, username)
	if err != nil {
		return "", err
	}

	for _, u := range users {
		uname, ok := u["username"]
		if !ok {
			return "", errors.New("couldn't find user")
		}
		if uname == username {
			typed, err := u.ToSSOUserTyped()
			if err != nil {
				return "", errors.Wrap(err, "couldn't convert user")
			}
			return typed.Email, nil
		}
	}
	return "", errors.New("couldn't find user")
}

func (s *Service) GetUser(ctx context.Context, username string) (SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "GetUser")
	defer span.End()

	users, err := s.getUser(ctxLocal, username)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		uname, ok := u["username"]
		if !ok {
			return nil, errors.New("couldn't find user")
		}
		if uname == username {
			return u, nil
		}
	}
	return nil, errors.New("couldn't find user")
}

func (s *Service) GetUsers(ctx context.Context, username string, limit *int, filter []string) ([]SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "GetUser")
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

func (s *Service) getUser(ctx context.Context, search string) ([]SSOUser, error) {
	search = strings.TrimSpace(search)
	if search == "" {
		return make([]SSOUser, 0), nil
	}

	ctxLocal, span := trace.StartSpan(ctx, "getUser")
	defer span.End()

	var req *http.Request
	var err error

	req, err = http.NewRequestWithContext(ctxLocal, http.MethodGet, s.searchUrl, http.NoBody)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()

	f := strings.Replace(usernameEqFilter, usernamePH, search, 1)
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
	var res SearchUsersResp
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res.Resources, nil
}

func (s *Service) getUsers(ctx context.Context, search string, limit int, filter []string) ([]SSOUser, error) {
	search = strings.TrimSpace(search)
	if search == "" {
		return make([]SSOUser, 0), nil
	}

	ctxLocal, span := trace.StartSpan(ctx, "getUsers")
	defer span.End()

	var req *http.Request
	var err error

	req, err = http.NewRequestWithContext(ctxLocal, http.MethodGet, s.searchUrl, http.NoBody)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	if limit > 100 {
		limit = 100
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
	var res SearchUsersResp
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}
	return res.Resources, nil
}
