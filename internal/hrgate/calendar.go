package hrgate

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"go.opencensus.io/trace"
)

const (
	limit = 100
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
func (s *Service) GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendar_days")
	defer span.End()

	res := CalendarDays{
		Holidays:    make([]int64, 0),
		PreHolidays: make([]int64, 0),
		WorkDay:     make([]int64, 0),
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
			switch *d.DayType {
			case CalendarDayTypePreHoliday:
				res.PreHolidays = append(res.PreHolidays, d.Date.Unix())
			case CalendarDayTypeHoliday:
				res.Holidays = append(res.Holidays, d.Date.Unix())
			case CalendarDayTypeWorkday:
				res.WorkDay = append(res.WorkDay, d.Date.Unix())
			default:
				return nil, fmt.Errorf("unknown day type: %s", *d.DayType)
			}

		} else {
			res.WorkDay = append(res.WorkDay, d.Date.Unix())
		}
	}

	sort.Slice(res.Holidays, func(i, j int) bool {
		return res.Holidays[i] < res.Holidays[j]
	})
	sort.Slice(res.WorkDay, func(i, j int) bool {
		return res.WorkDay[i] < res.WorkDay[j]
	})
	sort.Slice(res.PreHolidays, func(i, j int) bool {
		return res.PreHolidays[i] < res.PreHolidays[j]
	})

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
