package human_tasks

import (
	c "context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opencensus.io/plugin/ocgrpc"

	delegationht "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

const FromLoginFilter = "fromLogin"
const FromLoginsFilter = "fromLogins"
const ToLoginFilter = "toLogin"

type Service struct {
	c   *grpc.ClientConn
	cli delegationht.DelegationServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	client := delegationht.NewDelegationServiceClient(conn)

	return &Service{
		c:   conn,
		cli: client,
	}, nil
}

func (s *Service) getDelegationsInternal(ctx c.Context, req *delegationht.GetDelegationsRequest) (
	delegations Delegations, err error) {
	res, reqErr := s.cli.GetDelegations(ctx, req)
	if reqErr != nil {
		return nil, reqErr
	}

	for _, delegation := range res.Delegations {
		fromDate, parseFromDateErr := time.Parse(time.RFC3339, delegation.FromDate)
		if parseFromDateErr != nil {
			return nil, parseFromDateErr
		}

		toDate, parseToDateErr := time.Parse(time.RFC3339, delegation.ToDate)
		if parseToDateErr != nil {
			return nil, parseToDateErr
		}

		if time.Now().Before(toDate) {
			delegations = append(delegations, Delegation{
				FromDate:  fromDate,
				ToDate:    toDate,
				FromLogin: delegation.FromUser.Username,
				ToLogin:   delegation.ToUser.Username,
			})
		}
	}

	return delegations, nil
}

func (s *Service) GetDelegationsFromLogin(ctx c.Context, login string) (d Delegations, err error) {
	var req = &delegationht.GetDelegationsRequest{
		FilterBy:  FromLoginFilter,
		FromLogin: login,
	}

	res, err := s.getDelegationsInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsToLogin(ctx c.Context, login string) (d Delegations, err error) {
	var req = &delegationht.GetDelegationsRequest{
		FilterBy: ToLoginFilter,
		ToLogin:  login,
	}

	res, err := s.getDelegationsInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsByLogins(ctx c.Context, logins []string) (d Delegations, err error) {
	var sb strings.Builder

	for i, login := range logins {
		sb.WriteString(login)

		if i < len(logins)-1 {
			sb.WriteString(",")
		}
	}

	var req = &delegationht.GetDelegationsRequest{
		FilterBy:   FromLoginsFilter,
		FromLogins: sb.String(),
	}

	res, err := s.getDelegationsInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
