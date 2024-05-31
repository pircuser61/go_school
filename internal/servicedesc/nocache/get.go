package nocache

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	sd "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

func (s *service) GetSsoPerson(ctx c.Context, username string) (*sd.SsoPerson, error) {
	ctxLocal, span := trace.StartSpan(ctx, "servicedesc.get_sso_person")
	defer span.End()

	if sso.IsServiceUserName(username) {
		return &sd.SsoPerson{
			Username: username,
		}, nil
	}

	reqURL := fmt.Sprintf("%s%s", s.sdURL, fmt.Sprintf(getUserInfo, username))

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code from sso: %d, username: %s", resp.StatusCode, username)
	}

	res := &sd.SsoPerson{}
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res, nil
}

//nolint:dupl //its not duplicate
func (s *service) GetWorkGroup(ctx c.Context, groupID string) (*sd.WorkGroup, error) {
	ctxLocal, span := trace.StartSpan(ctx, "servicedesc.get_work_group")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.sdURL, getWorkGroup, groupID)

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
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

	res := &sd.WorkGroup{}
	if unmErr := json.NewDecoder(resp.Body).Decode(res); unmErr != nil {
		return nil, unmErr
	}

	log := logger.GetLogger(ctx)
	log.Info(fmt.Sprintf("got %d from group: %s", len(res.People), res.GroupName))

	return res, nil
}

func (s *service) GetSchemaByID(ctx c.Context, schemaID string) (map[string]interface{}, error) {
	ctxLocal, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.sdURL, getSchemaByID, schemaID)

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
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

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}

func (s *service) GetSchemaByBlueprintID(ctx c.Context, blueprintID string) (map[string]interface{}, error) {
	ctxLocal, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_blueprint_id")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s%s", s.sdURL, getSchemaByBlueprintID, blueprintID, "/json")

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
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

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}
