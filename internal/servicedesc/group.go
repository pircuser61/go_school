package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

const (
	getWorkGroup = "/api/chainsmith/v1/workGroup/"
)

type GroupMember struct {
	Login string `json:"login"`
}

type WorkGroup struct {
	GroupID   string        `json:"groupID"`
	GroupName string        `json:"groupName"`
	People    []GroupMember `json:"people"`
}

//nolint:dupl //its not duplicate
func (s *Service) GetWorkGroup(ctx context.Context, groupID string) (*WorkGroup, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_work_group")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s%s", s.SdURL, getWorkGroup, groupID)

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

	res := &WorkGroup{}
	if unmErr := json.NewDecoder(resp.Body).Decode(res); unmErr != nil {
		return nil, unmErr
	}

	log := logger.GetLogger(ctx)
	log.Info(fmt.Sprintf("got %d from group: %s", len(res.People), res.GroupName))

	return res, nil
}
