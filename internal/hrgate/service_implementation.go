package hrgate

import (
	c "context"
	"fmt"
	"math"
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	RussianFederation = "Российская Федерация"
	defaultLogin      = "gvshestako"
)

type Service struct {
	hrGateURL             string
	DefaultCalendarUnitID *string
	location              time.Location
	cli                   *ClientWithResponses
	cliPing               *http.Client
	Cache                 cachekit.Cache
}

func NewService(cfg *Config, ssoS *sso.Service, m metrics.Metrics) (ServiceInterface, error) {
	httpClient := &http.Client{}
	httpClient.Transport = &transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:     ssoS,
		scope:   cfg.Scope,
		metrics: m,
	}

	retryableCli := httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay)
	wrappedRetryableCli := httpRequestDoer{retryableCli}

	newCli, err := NewClientWithResponses(cfg.HRGateURL, WithHTTPClient(wrappedRetryableCli), WithBaseURL(cfg.HRGateURL))
	if err != nil {
		return nil, err
	}

	location, getLocationErr := time.LoadLocation("Europe/Moscow")
	if getLocationErr != nil {
		return nil, getLocationErr
	}

	return &Service{
		cli:       newCli,
		hrGateURL: cfg.HRGateURL,
		location:  *location,
		cliPing:   &http.Client{Timeout: time.Second * 2},
	}, nil
}

func (s *Service) Ping(ctx c.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", s.hrGateURL, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := s.cliPing.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("wrong status code: %d", resp.StatusCode)
	}

	return resp.Body.Close()
}

func (s *Service) GetCalendars(ctx c.Context, params *GetCalendarsParams) ([]Calendar, error) {
	ctxLocal, span := trace.StartSpan(ctx, "hrgate.get_calendars")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")
	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	response, err := s.cli.GetCalendarsWithResponse(ctxLocal, params)
	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to hrgate. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to hrgate: ", attempt)
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code on getting calendars: %d", response.StatusCode())
	}

	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get calendars by unit ids")
	}

	return *response.JSON200, err
}

func (s *Service) GetCalendarDays(ctx c.Context, params *GetCalendarDaysParams) (*CalendarDays, error) {
	ctxLocal, span := trace.StartSpan(ctx, "hrgate.get_calendar_days")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")
	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)
	res := CalendarDays{
		CalendarMap: make(map[int64]CalendarDayType),
	}

	resp, err := s.cli.GetCalendarDaysWithResponse(ctxLocal, params)
	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to hrgate. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to hrgate: ", attempt)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid code on getting calendar days: %d", resp.StatusCode())
	}

	for i := range *resp.JSON200 {
		d := (*resp.JSON200)[i]
		if d.DayType != nil {
			res.CalendarMap[d.Date.Unix()] = *d.DayType
		} else {
			res.CalendarMap[d.Date.Unix()] = CalendarDayTypeWorkday
		}
	}

	return &res, nil
}

func (s *Service) GetPrimaryRussianFederationCalendarOrFirst(ctx c.Context, params *GetCalendarsParams) (*Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_primary_calendar_or_first")
	defer span.End()

	calendars, getCalendarsErr := s.GetCalendars(ctx, params)

	if getCalendarsErr != nil {
		return nil, getCalendarsErr
	}

	for calendarIdx := range calendars {
		calendar := calendars[calendarIdx]

		if calendar.Primary != nil && *calendar.Primary && calendar.HolidayCalendar == RussianFederation {
			return &calendar, nil
		}
	}

	return &calendars[0], nil
}

func (s *Service) FillDefaultUnitID(ctx c.Context) error {
	ctx, span := trace.StartSpan(ctx, "hrgate.fill_default_unit_id")
	defer span.End()

	employee, err := s.GetEmployeeByLogin(ctx, defaultLogin)
	if err != nil {
		return err
	}

	if employee.OrganizationId == nil {
		return fmt.Errorf("cant get organization id by login: %s", defaultLogin)
	}

	organization, err := s.GetOrganizationByID(ctx, *employee.OrganizationId)
	if err != nil {
		return err
	}

	if organization.Unit == nil {
		return fmt.Errorf("cant get ogranization unit id by login: %s", defaultLogin)
	}

	s.DefaultCalendarUnitID = (*string)(&organization.Unit.Id)

	return nil
}

func (s *Service) GetDefaultUnitID() string {
	return *s.DefaultCalendarUnitID
}

// nolint:dupl //так нужно!
func (s *Service) GetDefaultCalendarDaysForGivenTimeIntervals(
	ctx c.Context,
	taskTimeIntervals []entity.TaskCompletionInterval,
) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_default_calendar_days_for_given_time_intervals")
	defer span.End()

	unitID := s.GetDefaultUnitID()

	calendar, getCalendarsErr := s.GetPrimaryRussianFederationCalendarOrFirst(ctx, &GetCalendarsParams{
		UnitIDs: &UnitIDs{unitID},
	})

	if getCalendarsErr != nil {
		return nil, getCalendarsErr
	}

	minIntervalTime, err := utils.FindMin(taskTimeIntervals, func(a, b entity.TaskCompletionInterval) bool {
		return a.StartedAt.Unix() < b.StartedAt.Unix()
	})
	if err != nil {
		return nil, err
	}

	minIntervalTime.StartedAt = minIntervalTime.StartedAt.Add(-time.Hour * 24 * 7)

	maxIntervalTime, err := utils.FindMax(taskTimeIntervals, func(a, b entity.TaskCompletionInterval) bool {
		return a.StartedAt.Unix() < b.StartedAt.Unix()
	})
	if err != nil {
		return nil, err
	}

	maxIntervalTime.FinishedAt = minIntervalTime.FinishedAt.Add(time.Hour * 24 * 7) // just taking more time

	calendarDays, getCalendarDaysErr := s.GetCalendarDays(ctx, &GetCalendarDaysParams{
		QueryFilters: &QueryFilters{
			WithDeleted: utils.GetAddressOfValue(false),
			Limit: utils.GetAddressOfValue(int(math.Ceil(utils.GetDateUnitNumBetweenDates(minIntervalTime.StartedAt,
				maxIntervalTime.FinishedAt, utils.Day)))),
		},
		Calendar: &IDsList{string(calendar.Id)},
		DateFrom: &openapi_types.Date{Time: minIntervalTime.StartedAt},
		DateTo:   &openapi_types.Date{Time: maxIntervalTime.FinishedAt},
	})
	if getCalendarDaysErr != nil {
		return nil, err
	}

	return calendarDays, nil
}

func (s *Service) GetEmployeeByLogin(ctx c.Context, username string) (*Employee, error) {
	ctxLocal, span := trace.StartSpan(ctx, "hrgate.get_employee_by_login")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")
	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	response, err := s.cli.GetEmployeesWithResponse(ctxLocal, &GetEmployeesParams{
		Logins: &[]string{username},
	})

	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to hrgate. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to hrgate: ", attempt)
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code on gettings employee by login: %d", response.StatusCode())
	}

	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get employee by login")
	}

	return &(*response.JSON200)[0], err
}

func (s *Service) GetOrganizationByID(ctx c.Context, organizationID string) (*Organization, error) {
	ctxLocal, span := trace.StartSpan(ctx, "hrgate.get_organization_by_id")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "HTTP")
	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	response, err := s.cli.GetOrganizationsIdWithResponse(ctxLocal, UUIDPathObjectID(organizationID))
	attempt := script.GetRetryCnt(ctxLocal) - 1

	if err != nil {
		log.Warning("Pipeliner failed to connect to hrgate. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to hrgate: ", attempt)
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code on gettings organization on id: %d", response.StatusCode())
	}

	return response.JSON200, nil
}

func (s *Service) GetLocation() time.Location {
	return s.location
}
