package nocache

import (
	c "context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (s *service) GetUserEmail(ctx c.Context, username string) (string, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_user_email")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	email, err := s.iga.GetUserEmail(ctxLocal, username)
	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to iga. Exceeded max retry count: ", attempt)

		return "", err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to iga: ", attempt)
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

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	igaSsoUser, err := s.iga.GetUser(ctxLocal, username, onlyEnabled)
	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to iga. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to iga: ", attempt)
	}

	res := people.SSOUser(igaSsoUser)

	return res, nil
}

func (s *service) GetUsers(ctx c.Context, username string, limit *int, filter []string, enabled bool) ([]people.SSOUser, error) {
	ctxLocal, span := trace.StartSpan(ctx, "people.nocache.get_users")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	igaSsoUsers, err := s.iga.GetUsers(ctxLocal, username, limit, filter, enabled)
	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to iga. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to iga: ", attempt)
	}

	res := make([]people.SSOUser, 0)

	for i := range igaSsoUsers {
		ssoUser := people.SSOUser(igaSsoUsers[i])
		res = append(res, ssoUser)
	}

	return res, err
}
