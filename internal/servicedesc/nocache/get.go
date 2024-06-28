package nocache

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	sd "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
)

const externalSystemName = "servicedesk"

//nolint:dupl //its not duplicate
func (s *service) GetWorkGroup(ctx c.Context, groupID string) (*sd.WorkGroup, error) {
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_work_group")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "HTTP").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	reqURL := fmt.Sprintf("%s%s%s", s.sdURL, getWorkGroup, groupID)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		script.LogRetryFailure(ctx, uint(s.cli.RetryMax))

		return nil, err
	}

	defer resp.Body.Close()

	script.LogRetrySuccess(ctx)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	res := &sd.WorkGroup{}
	if unmErr := json.NewDecoder(resp.Body).Decode(res); unmErr != nil {
		return nil, unmErr
	}

	log.Info(fmt.Sprintf("got %d from group: %s", len(res.People), res.GroupName))

	return res, nil
}

func (s *service) GetSchemaByID(ctx c.Context, schemaID string) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_id")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "HTTP").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	reqURL := fmt.Sprintf("%s%s%s", s.sdURL, getSchemaByID, schemaID)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		script.LogRetryFailure(ctx, uint(s.cli.RetryMax))

		return nil, err
	}

	defer resp.Body.Close()

	script.LogRetrySuccess(ctx)

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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_blueprint_id")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "HTTP").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)
	reqURL := fmt.Sprintf("%s%s%s%s", s.sdURL, getSchemaByBlueprintID, blueprintID, "/json")

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		script.LogRetryFailure(ctx, uint(s.cli.RetryMax))

		return nil, err
	}

	defer resp.Body.Close()

	script.LogRetrySuccess(ctx)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	schema := make(map[string]interface{})
	if unmErr := json.NewDecoder(resp.Body).Decode(&schema); unmErr != nil {
		return nil, unmErr
	}

	return schema, nil
}
