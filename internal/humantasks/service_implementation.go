package humantasks

import (
	c "context"
	"strings"
	"time"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
	"google.golang.org/grpc"
)

const (
	FromLoginFilter  = "fromLogin"
	FromLoginsFilter = "fromLogins"
	ToLoginFilter    = "toLogin"
	ToLoginsFilter   = "toLogins"
)

type Service struct {
	C     *grpc.ClientConn
	Cli   d.DelegationServiceClient
	Cache cachekit.Cache
}

func (s *Service) GetDelegations(ctx c.Context, req *d.GetDelegationsRequest) (ds Delegations, err error) {
	if s.Cli == nil || s.C == nil {
		return make([]Delegation, 0), nil
	}

	res, reqErr := s.Cli.GetDelegations(ctx, req)
	if reqErr != nil {
		return nil, reqErr
	}

	for _, delegation := range res.Delegations {
		fromDate, parseFromDateErr := time.Parse("02/01/2006", delegation.FromDate)
		if parseFromDateErr != nil {
			return nil, parseFromDateErr
		}

		toDate := time.Time{}

		if delegation.ToDate != "" {
			var parseToDateErr error

			toDate, parseToDateErr = time.Parse("02/01/2006", delegation.ToDate)
			if parseToDateErr != nil {
				return nil, parseToDateErr
			}
		}

		if (time.Now().After(toDate) && !toDate.IsZero()) || time.Now().Before(fromDate) {
			continue
		}

		ds = append(ds, Delegation{
			FromDate:        fromDate,
			ToDate:          toDate,
			FromLogin:       delegation.FromUser.Username,
			ToLogin:         delegation.ToUser.Username,
			DelegationTypes: delegation.DelegationTypes,
		})
	}

	return ds, nil
}

func (s *Service) GetDelegationsFromLogin(ctx c.Context, login string) (ds Delegations, err error) {
	req := &d.GetDelegationsRequest{
		FilterBy:  FromLoginFilter,
		FromLogin: login,
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsToLogin(ctx c.Context, login string) (ds Delegations, err error) {
	req := &d.GetDelegationsRequest{
		FilterBy: ToLoginFilter,
		ToLogin:  login,
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsToLogins(ctx c.Context, logins []string) (ds Delegations, err error) {
	var sb strings.Builder

	for i, login := range logins {
		sb.WriteString(login)

		if i < len(logins)-1 {
			sb.WriteString(",")
		}
	}

	req := &d.GetDelegationsRequest{
		FilterBy: ToLoginsFilter,
		ToLogins: sb.String(),
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsByLogins(ctx c.Context, logins []string) (ds Delegations, err error) {
	var sb strings.Builder

	for i, login := range logins {
		sb.WriteString(login)

		if i < len(logins)-1 {
			sb.WriteString(",")
		}
	}

	req := &d.GetDelegationsRequest{
		FilterBy:   FromLoginsFilter,
		FromLogins: sb.String(),
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
