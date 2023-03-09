package hrgate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"go.opencensus.io/trace"
)

const (
	defaultEmployeeLogin        = "voronin"
	getCalendarByUnitId         = "/calendars"
	getEmployeeByLogin          = "/employees"
	getOrganizationById         = "/organizations/%s"
	getCalendarDaysByCalendarId = "/calendar-days"
	limit                       = 500
	totalHeader                 = "total"
	offsetHeader                = "offset"
	limitHeader                 = "limit"
)

type UnitIDs []string

type GetCalendarsParams struct {

	// список id юнитов для фильтрации
	UnitIDs *UnitIDs
}

type IDsList []string

type GetCalendarDaysParams struct {
	// фильтр по id календарей
	Calendars *IDsList
}

func handleHeaders(hh http.Header) (total, offset, limit int, err error) {
	currTotal := hh.Get(totalHeader)
	total, err = strconv.Atoi(currTotal)
	if err != nil {
		return 0, 0, 0, err
	}

	currOffset := hh.Get(offsetHeader)
	offset, err = strconv.Atoi(currOffset)
	if err != nil {
		return 0, 0, 0, err
	}

	currLimit := hh.Get(limitHeader)
	limit, err = strconv.Atoi(currLimit)
	if err != nil {
		return 0, 0, 0, err
	}

	return
}

func (s *Service) GetEmployeeByLogin(ctx context.Context, username string) (*Employee, error) {
	ctx, span := trace.StartSpan(ctx, "get_employee_by_login")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s", s.HrGateUrl, getEmployeeByLogin)
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if reqErr != nil {
		return &Employee{}, reqErr
	}

	q := req.URL.Query()
	q.Add("logins", username)
	req.URL.RawQuery = q.Encode()

	resp, doErr := s.Cli.Do(req)
	if doErr != nil {
		return &Employee{}, doErr
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Employee{}, fmt.Errorf("got bad status code on getting employee by login: %d", resp.StatusCode)
	}
	data, readAllErr := io.ReadAll(resp.Body)
	if readAllErr != nil {
		return &Employee{}, readAllErr
	}

	var employees Employees

	unmarshalErr := json.Unmarshal(data, &employees)
	if unmarshalErr != nil {
		return &Employee{}, unmarshalErr
	}

	if len(employees) != 1 {
		return &Employee{}, fmt.Errorf("cant get employee by login %s", username)
	}

	return employees[0], nil
}

func (s *Service) GetOrganizationById(ctx context.Context, organizationId uuid.UUID) (*Organization, error) {
	ctx, span := trace.StartSpan(ctx, "get_organization_by_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s", s.HrGateUrl, fmt.Sprintf(getOrganizationById, organizationId.String()))
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if reqErr != nil {
		return &Organization{}, reqErr
	}

	resp, doErr := s.Cli.Do(req)
	if doErr != nil {
		return &Organization{}, doErr
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Organization{}, fmt.Errorf("got bad status code on getting organization by id: %d", resp.StatusCode)
	}
	data, readAllErr := io.ReadAll(resp.Body)
	if readAllErr != nil {
		return &Organization{}, readAllErr
	}

	var organization Organization

	unmarshalErr := json.Unmarshal(data, &organization)
	if unmarshalErr != nil {
		return &Organization{}, unmarshalErr
	}

	return &organization, nil
}

func (s *Service) GetCalendars(ctx context.Context, params *GetCalendarsParams) (Calendars, error) {
	ctx, span := trace.StartSpan(ctx, "get_calendars")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s", s.HrGateUrl, getCalendarByUnitId)
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if reqErr != nil {
		return Calendars{}, reqErr
	}

	q := req.URL.Query()

	if params.UnitIDs != nil {
		for _, unitId := range *params.UnitIDs {
			q.Add("unitIDs", unitId)
		}
	}

	req.URL.RawQuery = q.Encode()

	resp, doErr := s.Cli.Do(req)
	if doErr != nil {
		return Calendars{}, doErr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Calendars{}, fmt.Errorf("got bad status code on getting calendars: %d", resp.StatusCode)
	}
	data, readAllErr := io.ReadAll(resp.Body)
	if readAllErr != nil {
		return Calendars{}, readAllErr
	}

	var calendars Calendars

	if err := json.Unmarshal(data, &calendars); err != nil {
		return Calendars{}, err
	}

	return calendars, nil
}

func (s *Service) GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*CalendarDays, error) {
	ctx, span := trace.StartSpan(ctx, "get_calendar_days")
	defer span.End()

	lim := limit
	offset := 0
	total := -1
	reqURL := fmt.Sprintf("%s%s", s.HrGateUrl, getCalendarDaysByCalendarId)
	var calendarDays CalendarDays
	for total == -1 || offset <= total {
		var handleErr error

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
		if reqErr != nil {
			return &CalendarDays{}, reqErr
		}

		q := req.URL.Query()

		for _, calendar := range *params.Calendars {
			q.Add("calendar", calendar)
		}

		q.Add("limit", strconv.FormatInt(int64(lim), 10))
		q.Add("offset", strconv.FormatInt(int64(offset), 10))
		req.URL.RawQuery = q.Encode()

		resp, doErr := s.Cli.Do(req)

		if doErr != nil {
			return &CalendarDays{}, doErr
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return &CalendarDays{}, fmt.Errorf("got bad status code on getting calendars: %d", resp.StatusCode)
		}
		data, readAllErr := io.ReadAll(resp.Body)

		if readAllErr != nil {
			resp.Body.Close()
			return &CalendarDays{}, readAllErr
		}

		responseCalendarDays := make(CalendarDays, 0, lim)

		if err := json.Unmarshal(data, &responseCalendarDays); err != nil {
			return &CalendarDays{}, err
		}

		calendarDays = append(calendarDays, calendarDays...)

		total, offset, lim, handleErr = handleHeaders(resp.Header)
		if handleErr != nil {
			resp.Body.Close()
			return &CalendarDays{}, handleErr
		}
		offset += lim

		resp.Body.Close()
	}

	return &calendarDays, nil
}

func (s *Service) FillDefaultUnitId(ctx context.Context) error {
	employee, err := s.GetEmployeeByLogin(ctx, defaultEmployeeLogin)
	if err != nil {
		return err
	}

	organization, err := s.GetOrganizationById(ctx, employee.OrganizationId)
	if err != nil {
		return err
	}

	s.DefaultCalendarUnitId = &organization.Unit.Id

	return nil
}

func (s *Service) GetDefaultCalendarDays(ctx context.Context) (*CalendarDays, error) {
	calendars, getCalendarsErr := s.GetCalendars(ctx, &GetCalendarsParams{UnitIDs: &UnitIDs{s.DefaultCalendarUnitId.String()}})

	if getCalendarsErr != nil {
		return &CalendarDays{}, getCalendarsErr
	}
	if len(calendars) != 1 {
		return &CalendarDays{}, fmt.Errorf("cant get default calendar days")
	}

	return s.GetCalendarDays(ctx, &GetCalendarDaysParams{Calendars: &IDsList{calendars[0].Id.String()}})
}
