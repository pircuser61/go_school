package nocache

import (
	c "context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (s *service) GetUserEmail(ctx c.Context, username string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "people.nocache.get_user_email")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "HTTP").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	email, err := s.iga.GetUserEmail(ctx, username)
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return "", err
	}

	script.LogRetrySuccess(ctx)

	return email, nil
}

type FindUserError struct {
	UserName string
}

func (e *FindUserError) Error() string {
	return "couldn't find user with name " + e.UserName
}

func (s *service) GetUser(ctx c.Context, username string, onlyEnabled bool) (people.SSOUser, error) {
	ctx, span := trace.StartSpan(ctx, "people.nocache.get_user")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "HTTP").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	igaSsoUser, err := s.iga.GetUser(ctx, username, onlyEnabled)
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return nil, err
	}

	script.LogRetrySuccess(ctx)

	res := people.SSOUser(igaSsoUser)

	return res, nil
}

func (s *service) GetUsers(ctx c.Context, username string, limit *int, filter []string, enabled bool) ([]people.SSOUser, error) {
	ctx, span := trace.StartSpan(ctx, "people.nocache.get_users")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "HTTP").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	igaSsoUsers, err := s.iga.GetUsers(ctx, username, limit, filter, enabled)
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return nil, err
	}

	script.LogRetrySuccess(ctx)

	res := make([]people.SSOUser, 0)

	for i := range igaSsoUsers {
		ssoUser := people.SSOUser(igaSsoUsers[i])
		res = append(res, ssoUser)
	}

	return res, err
}
