package hrgate

import (
	"context"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"
)

const (
	defaultLogin = "gvshestako"
)

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
		return nil, fmt.Errorf("invalid response code on gettings employee by login: %d", response.StatusCode())
	}

	if len(*response.JSON200) == 0 {
		return nil, fmt.Errorf("cant get employee by login")
	}

	return &(*response.JSON200)[0], err
}
