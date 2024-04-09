package hrgate

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"net/http"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/observability"
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type ServiceInterface interface {
	GetCalendars(ctx context.Context, params *GetCalendarsParams) ([]Calendar, error)
	GetPrimaryRussianFederationCalendarOrFirst(ctx context.Context, params *GetCalendarsParams) (*Calendar, error)
	GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error)
	FillDefaultUnitID(ctx context.Context) error
	GetDefaultUnitID() string
	GetDefaultCalendar(ctx context.Context) (*Calendar, error)
	GetDefaultCalendarDaysForGivenTimeIntervals(ctx context.Context, taskTimeIntervals []entity.TaskCompletionInterval) (*CalendarDays, error)
}

type ServiceWithCache struct {
	Cache  cachekit.Cache
	HRGate Service
}

type Service struct {
	HRGateURL             string
	DefaultCalendarUnitID *string
	Location              time.Location
	Cli                   *ClientWithResponses
	Cache                 cachekit.Cache
}

func NewService(cfg *Config, ssoS *sso.Service) (*Service, error) {
	httpClient := &http.Client{}
	tr := TransportForHrGate{
		transport: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}
	httpClient.Transport = &tr

	newCli, createClientErr := NewClientWithResponses(cfg.HRGateURL, WithHTTPClient(httpClient), WithBaseURL(cfg.HRGateURL))
	if createClientErr != nil {
		return nil, createClientErr
	}

	location, getLocationErr := time.LoadLocation("Europe/Moscow")
	if getLocationErr != nil {
		return nil, getLocationErr
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(CacheConfig{
		TTL:     time.Second * 5,
		Type:    "",
		Address: "redis-pipeliner:6379",
		DB:      0,
		Pass:    "pass",
	}))
	if cacheErr != nil {
		return nil, cacheErr
	}

	s := &Service{
		Cli:       newCli,
		HRGateURL: cfg.HRGateURL,
		Location:  *location,
		Cache:     cache,
	}

	return s, nil
}

type TransportForHrGate struct {
	transport ochttp.Transport
	sso       *sso.Service
	scope     string
}

func (t *TransportForHrGate) RoundTrip(req *http.Request) (*http.Response, error) {
	err := t.sso.BindAuthHeader(req.Context(), req, t.scope)
	if err != nil {
		return nil, err
	}

	return t.transport.RoundTrip(req)
}

func (s *Service) GetLocation() time.Location {
	return s.Location
}
