package humantasks

import (
	c "context"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const (
	FromLoginFilter  = "fromLogin"
	FromLoginsFilter = "fromLogins"
	ToLoginFilter    = "toLogin"
	ToLoginsFilter   = "toLogins"

	externalSystemName = "human-tasks"
)

type service struct {
	conn *grpc.ClientConn
	cli  d.DelegationServiceClient
}

func NewService(cfg *Config, log logger.Logger, m metrics.Metrics) (ServiceInterface, error) {
	if cfg.URL == "" {
		return &ServiceWithCache{}, nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithUnaryInterceptor(metrics.GrpcMetrics(externalSystemName, m)),
	}

	if cfg.MaxRetries != 0 {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(cfg.MaxRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(cfg.RetryDelay)),
			grpc_retry.WithPerRetryTimeout(cfg.Timeout),
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to humantasks")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	return &service{
		conn: conn,
		cli:  d.NewDelegationServiceClient(conn),
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	return nil
}

func (s *service) SetCli(cli d.DelegationServiceClient) {
	s.cli = cli
}

func (s *service) GetDelegations(ctx c.Context, req *d.GetDelegationsRequest) (ds Delegations, err error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations")
	defer span.End()

	if s.cli == nil || s.conn == nil {
		return make([]Delegation, 0), nil
	}

	res, reqErr := s.cli.GetDelegations(ctx, req)
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

func (s *service) GetDelegationsFromLogin(ctx c.Context, login string) (ds Delegations, err error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_from_login")
	defer span.End()

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

func (s *service) GetDelegationsToLogin(ctx c.Context, login string) (ds Delegations, err error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_to_login")
	defer span.End()

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

func (s *service) GetDelegationsToLogins(ctx c.Context, logins []string) (ds Delegations, err error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_to_logins")
	defer span.End()

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

func (s *service) GetDelegationsByLogins(ctx c.Context, logins []string) (ds Delegations, err error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_by_logins")
	defer span.End()

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
