package hrgate

import (
	"context"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"
)

func (s *Service) GetOrganizationByID(ctx context.Context, organizationID string) (*Organization, error) {
	ctx, span := trace.StartSpan(ctx, "hrgate.get_organization_by_id")
	defer span.End()

	response, err := s.Cli.GetOrganizationsIdWithResponse(ctx, UUIDPathObjectID(organizationID))
	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid response code on gettings organization on id: %d", response.StatusCode())
	}

	return response.JSON200, nil
}
