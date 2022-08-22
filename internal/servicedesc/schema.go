package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"
)

func (s *Service) GetSchemaFieldsByApplication(ctx context.Context, applicationID string) (map[string]string, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_schema_fields_by_application")
	defer span.End()

	var req *http.Request
	var err error

	reqURL := fmt.Sprintf("%s%s", s.sdURL, fmt.Sprintf(getSchemaFieldsByApplication, applicationID))

	req, err = makeRequest(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	var res map[string]string
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res, nil
}
