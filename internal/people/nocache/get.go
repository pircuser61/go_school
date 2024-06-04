package nocache

import (
	c "context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
)

func (s *service) GetUserEmail(ctx c.Context, username string) (string, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_user_email")
	defer span.End()

	email, err := s.iga.GetUserEmail(ctxLocal, username)
	if err != nil {
		return "", err
	}

	return email, nil
}

type FindUserError struct {
	UserName string
}

func (e *FindUserError) Error() string {
	return "couldn't find user with name " + e.UserName
}

func (s *service) GetUser(ctx c.Context, username string, onlyEnabled bool) (people.SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_user")
	defer span.End()

	igaSsoUser, err := s.iga.GetUser(ctxLocal, username, onlyEnabled)
	if err != nil {
		return nil, err
	}

	res := make(people.SSOUser, len(igaSsoUser))
	for i := range igaSsoUser {
		res[i] = igaSsoUser[i]
	}

	return res, err
}

func (s *service) GetUsers(ctx c.Context, username string, limit *int, filter []string) ([]people.SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_users")
	defer span.End()

	igaSsoUsers, err := s.iga.GetUsers(ctxLocal, username, limit, filter)
	if err != nil {
		return nil, err
	}

	res := make([]people.SSOUser, 0)
	for i := range igaSsoUsers {
		ssoUser := make(people.SSOUser, len(igaSsoUsers[i]))
		for j := range igaSsoUsers[i] {
			ssoUser[j] = igaSsoUsers[i][j]
		}

		res = append(res, ssoUser)
	}

	return res, err
}
