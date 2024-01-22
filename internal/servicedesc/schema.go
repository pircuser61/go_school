package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"
)

const (
	getSchemaByID          = "/api/herald/v1/schema/"
	getSchemaByBlueprintID = "/api/herald/v1//schema/blueprint/"
)

func (s *Service) GetSchemaByID(ctx context.Context, schemaID string) (map[string]interface{}, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_schema_by_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getSchemaByID, schemaID)

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.Cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}

func (s *Service) GetSchemaByBlueprintID(ctx context.Context, blueprintID string) (map[string]interface{}, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_schema_by_blueprint_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getSchemaByBlueprintID, blueprintID)

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.Cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}
