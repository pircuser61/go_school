package people

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"go.opencensus.io/trace"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"net/url"
	"path"
	"regexp"
	"strings"
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

	sortByVal           = "lastName,firstName"
	companyFilterOption = `(attributes.OrgUnit co "%s")`
)

//nolint:gochecknoglobals // в данном проекте было бы проще отключить этот линтер
var (
	englishCheck = regexp.MustCompile(`[^а-яё]`)
	newStringRm  = regexp.MustCompile(`(\s\s+)|(\n)`)

	usernameEqFilter            = fmt.Sprintf("(username eq %q)", usernamePH)
	usernameEqOnlyEnabledFilter = fmt.Sprintf("((username eq %q) and (enabled eq true))", usernamePH)

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
)

// nolint:staticcheck // Cant use cases.Title
func defineFilter(input string, oneWord bool, filter []string) string {
	parts := strings.Split(strings.ToLower(input), " ")
	caser := cases.Title(language.Tag{})

	var q string

	if len(parts) == 1 || oneWord {
		if englishCheck.FindString(strings.ToLower(parts[0])) != "" {
			q = strings.Replace(usernameFilter, usernamePH, parts[0], 1)
		} else {
			q = strings.Replace(onePartFilter, name1PH, caser.String(parts[0]), 1)
		}
	}

	if len(parts) == 2 {
		q = strings.ReplaceAll(twoPartFilter, name1PH, caser.String(parts[0]))
		q = strings.ReplaceAll(q, name2PH, caser.String(parts[1]))
	}

	if len(parts) > 2 {
		q = strings.ReplaceAll(threePartFilter, name1PH, caser.String(parts[0]))
		q = strings.ReplaceAll(q, name2PH, caser.String(parts[1]))
		q = strings.ReplaceAll(q, name3PH, caser.String(parts[2]))
	}

	if len(filter) > 0 {
		companyOptions := make([]string, 0, len(filter))

		for _, f := range filter {
			companyOptions = append(companyOptions, fmt.Sprintf(companyFilterOption, f))
		}

		q += fmt.Sprintf(" and (%s)", strings.Join(companyOptions, " or "))
	}

	return newStringRm.ReplaceAllString(q, " ")
}

type PeopleInterface interface {
	pathBuilder(mainpath, subpath string) (string, error)
	GetUserEmail(ctx context.Context, username string) (string, error)
	GetUser(ctx context.Context, username string) (SSOUser, error)
	GetUsers(ctx context.Context, username string, limit *int, filter []string) ([]SSOUser, error)
	getUser(ctx context.Context, search string, onlyEnabled bool) ([]SSOUser, error)
	getUsers(ctx context.Context, search string, limit int, filter []string) ([]SSOUser, error)
}

func (s *Service) getUser(ctx context.Context, search string, onlyEnabled bool) ([]SSOUser, error) {
	keyForCache := "user" + ":" + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type []SSOUser")
		}

		return resources, nil
	}

	resources, err := s.getUser(ctx, search, onlyEnabled)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
}

func (s *Service) getUsers(ctx context.Context, search string, limit int, filter []string) ([]SSOUser, error) {
	keyForCache := "users" + ":" + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type []SSOUser")
		}

		return resources, nil
	}

	resources, err := s.getUsers(ctx, search, limit, filter)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
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

	if sso.IsServiceUserName(username) {
		return "", nil
	}

	users, err := s.getUser(ctxLocal, username, true)
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
