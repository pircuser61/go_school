package hrgate

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"go.opencensus.io/trace"
)

const (
	limit        = 500
	totalHeader  = "total"
	offsetHeader = "offset"
	limitHeader  = "limit"
	defaultLogin = "voronin"
)

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
	ctx, span := trace.StartSpan(ctx, "hrgate.get_employee_by_login")
	defer span.End()

	response, err := s.Cli.GetEmployeesWithResponse(ctx, &GetEmployeesParams{
		Logins: &[]string{username},
	})

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode())
	}
	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get employee by login")
	}

	return &(*response.JSON200)[0], err

}

func (s *Service) GetOrganizationById(ctx context.Context, organizationId string) (*Organization, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_organization_by_id")
	defer span.End()

	response, err := s.Cli.GetOrganizationsIdWithResponse(ctx, UUIDPathObjectID(organizationId))

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode())
	}

	return response.JSON200, nil
}

func (s *Service) GetCalendars(ctx context.Context, params *GetCalendarsParams) (*[]Calendar, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendars")
	defer span.End()

	response, err := s.Cli.GetCalendarsWithResponse(ctx, params)

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode())
	}
	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get calendars by unit ids")
	}

	return response.JSON200, err
}
func (s *Service) GetCalendarDays(ctx context.Context, params *GetCalendarDaysParams) (*[]CalendarDay, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_calendar_days")
	defer span.End()

	response, err := s.Cli.GetCalendarDaysWithResponse(ctx, params)

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %d", response.StatusCode())
	}
	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get calendar days")
	}

	return response.JSON200, err
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
