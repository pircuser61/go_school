package hrgate

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	RussianFederation = "Российская Федерация"
)

func (s *Service) GetCalendars(ctx context.Context, params *GetCalendarsParams) ([]Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendars")
	defer span.End()

	response, err := s.Cli.GetCalendarsWithResponse(ctx, params)

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code on getting calendars: %d", response.StatusCode())
	}
	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get calendars by unit ids")
	}

	return *response.JSON200, err
}

func (s *Service) GetPrimaryRussianFederationCalendarOrFirst(ctx context.Context, params *GetCalendarsParams) (*Calendar, error) {
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

func (s *Service) GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendar_days")
	defer span.End()

	res := CalendarDays{
		CalendarMap: make(map[int64]CalendarDayType),
	}

	resp, err := s.Cli.GetCalendarDaysWithResponse(ctx, params)
	if err != nil {
		return nil, err
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

func (s *Service) FillDefaultUnitId(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "hrgate.fill_default_unit_id")
	defer span.End()
	employee, err := s.GetEmployeeByLogin(ctx, defaultLogin)
	if err != nil {
		return err
	}

	if employee.OrganizationId == nil {
		return fmt.Errorf("cant get organization id by login: %s", defaultLogin)
	}

	organization, err := s.GetOrganizationById(ctx, *employee.OrganizationId)
	if err != nil {
		return err
	}

	if organization.Unit == nil {
		return fmt.Errorf("cant get ogranization unit id by login: %s", defaultLogin)
	}

	s.DefaultCalendarUnitId = (*string)(&organization.Unit.Id)

	return nil
}

func (s *Service) GetDefaultUnitId() string {
	return *s.DefaultCalendarUnitId
}

func (s *Service) GetDefaultCalendar(ctx context.Context) (*Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_default_calendar")
	defer span.End()

	unitId := s.GetDefaultUnitId()

	calendars, getCalendarsErr := s.GetCalendars(ctx, &GetCalendarsParams{
		QueryFilters: nil,
		UnitIDs:      &UnitIDs{unitId},
	})

	if getCalendarsErr != nil {
		return nil, getCalendarsErr
	}

	return &calendars[0], nil
}

func (s *Service) GetDefaultCalendarDaysForGivenTimeIntervals(
	ctx context.Context,
	taskTimeIntervals []entity.TaskCompletionInterval) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_default_calendar_days_for_given_time_intervals")
	defer span.End()

	unitId := s.GetDefaultUnitId()

	calendar, getCalendarsErr := s.GetPrimaryRussianFederationCalendarOrFirst(ctx, &GetCalendarsParams{
		UnitIDs: &UnitIDs{unitId},
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
